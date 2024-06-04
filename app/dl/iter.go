package dl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/iyear/tdl/pkg/iyzyi"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"text/template"
	"time"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/peers"

	"github.com/iyear/tdl/pkg/dcpool"
	"github.com/iyear/tdl/pkg/downloader"
	"github.com/iyear/tdl/pkg/tmedia"
	"github.com/iyear/tdl/pkg/tmessage"
	"github.com/iyear/tdl/pkg/tplfunc"
	"github.com/iyear/tdl/pkg/utils"
)

const tempExt = ".tmp"

type fileTemplate struct {
	DialogID     int64
	MessageID    int
	MessageDate  int64
	FileName     string
	FileCaption  string
	FileSize     string
	DownloadDate int64
}

type iter struct {
	pool    dcpool.Pool
	manager *peers.Manager
	dialogs []*tmessage.Dialog
	tpl     *template.Template
	include map[string]struct{}
	exclude map[string]struct{}
	opts    Options
	delay   time.Duration

	mu          *sync.Mutex
	finished    map[int]struct{}
	fingerprint string
	preSum      []int
	i, j        int
	elem        downloader.Elem
	err         error

	record *iyzyi.Recorder
}

func newIter(ctx context.Context, pool dcpool.Pool, manager *peers.Manager, dialog [][]*tmessage.Dialog,
	opts Options, delay time.Duration,
) (*iter, error) {
	record, err := iyzyi.NewRecorder()
	if err != nil {
		return nil, err
	}

	tpl, err := template.New("dl").
		Funcs(tplfunc.FuncMap(tplfunc.All...)).
		Parse(opts.Template)
	if err != nil {
		return nil, errors.Wrap(err, "parse template")
	}

	dialogs := flatDialogs(dialog)

	skip, err := iyzyi.RemoveRecordedMessages("download", ctx, manager, record, &dialogs)
	if err != nil {
		return nil, err
	}

	// if msgs is empty, return error to avoid range out of index
	if len(dialogs) == 0 {
		if skip {
			fmt.Printf("There are no messages to download after skipping recorded messages.\n")
			return nil, nil
		} else {
			return nil, errors.Errorf("you must specify at least one message")
		}
	}

	// include and exclude
	includeMap := filterMap(opts.Include, utils.FS.AddPrefixDot)
	excludeMap := filterMap(opts.Exclude, utils.FS.AddPrefixDot)

	// to keep fingerprint stable
	sortDialogs(dialogs, opts.Desc)

	return &iter{
		pool:    pool,
		manager: manager,
		dialogs: dialogs,
		opts:    opts,
		include: includeMap,
		exclude: excludeMap,
		tpl:     tpl,
		delay:   delay,

		mu:          &sync.Mutex{},
		finished:    make(map[int]struct{}),
		fingerprint: fingerprint(dialogs),
		preSum:      preSum(dialogs),
		i:           0,
		j:           0,
		elem:        nil,
		err:         nil,

		record: record,
	}, nil
}

func (i *iter) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		i.err = ctx.Err()
		return false
	default:
	}

	// if delay is set, sleep for a while for each iteration
	if i.delay > 0 && (i.i+i.j) > 0 { // skip first delay
		time.Sleep(i.delay)
	}

	for {
		ok, skip := i.process(ctx)
		if skip {
			continue
		}

		return ok
	}
}

func (i *iter) process(ctx context.Context) (ret bool, skip bool) {
	i.mu.Lock()
	defer i.mu.Unlock()

	defer func() {
		if i.j++; i.i < len(i.dialogs) && i.j >= len(i.dialogs[i.i].Messages) {
			i.i++
			i.j = 0
		}
	}()

	// end of iteration or error occurred
	if i.i >= len(i.dialogs) || i.j >= len(i.dialogs[i.i].Messages) || i.err != nil {
		return false, false
	}

	peer, msg := i.dialogs[i.i].Peer, i.dialogs[i.i].Messages[i.j]

	// check if finished
	if _, ok := i.finished[i.ij2n(i.i, i.j)]; ok {
		return false, true
	}

	from, err := i.manager.FromInputPeer(ctx, peer)
	if err != nil {
		i.err = errors.Wrap(err, "resolve from input peer")
		return false, false
	}

	message, err := utils.Telegram.GetSingleMessage(ctx, i.pool.Default(ctx), peer, msg)
	if err != nil {
		i.err = errors.Wrap(err, "resolve message")
		return false, false
	}

	item, ok := tmedia.GetMedia(message)
	if !ok {
		i.err = errors.Errorf("can not get media from %d/%d message", from.ID(), message.ID)
		return false, false
	}

	// process include and exclude
	ext := filepath.Ext(item.Name)
	if _, ok = i.include[ext]; len(i.include) > 0 && !ok {
		return false, true
	}
	if _, ok = i.exclude[ext]; len(i.exclude) > 0 && ok {
		return false, true
	}

	toName := bytes.Buffer{}
	err = i.tpl.Execute(&toName, &fileTemplate{
		DialogID:     from.ID(),
		MessageID:    message.ID,
		MessageDate:  int64(message.Date),
		FileName:     item.Name,
		FileCaption:  message.Message,
		FileSize:     utils.Byte.FormatBinaryBytes(item.Size),
		DownloadDate: time.Now().Unix(),
	})
	if err != nil {
		i.err = errors.Wrap(err, "execute template")
		return false, false
	}

	if i.opts.SkipSame {
		if stat, err := os.Stat(filepath.Join(i.opts.Dir, toName.String())); err == nil {
			if utils.FS.GetNameWithoutExt(toName.String()) == utils.FS.GetNameWithoutExt(stat.Name()) &&
				stat.Size() == item.Size {
				return false, true
			}
		}
	}

	filename := fmt.Sprintf("%s%s", toName.String(), tempExt)
	path := filepath.Join(i.opts.Dir, filename)

	// #113. If path contains dirs, create it. So now we support nested dirs.
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		i.err = errors.Wrap(err, "create dir")
		return false, false
	}

	to, err := os.Create(path)
	if err != nil {
		i.err = errors.Wrap(err, "create file")
		return false, false
	}

	i.elem = &iterElem{
		id: i.ij2n(i.i, i.j),

		from:    from,
		fromMsg: message,
		file:    item,

		to: to,

		opts: i.opts,
	}

	return true, false
}

func (i *iter) Value() downloader.Elem {
	return i.elem
}

func (i *iter) Err() error {
	return i.err
}

func (i *iter) Record() *iyzyi.Recorder {
	return i.record
}

func (i *iter) SetFinished(finished map[int]struct{}) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.finished = finished
}

func (i *iter) Finished() map[int]struct{} {
	i.mu.Lock()
	defer i.mu.Unlock()

	return i.finished
}

func (i *iter) Fingerprint() string {
	return i.fingerprint
}

func (i *iter) Finish(id int) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.finished[id] = struct{}{}
}

func (i *iter) Total() int {
	i.mu.Lock()
	defer i.mu.Unlock()

	total := 0
	for _, m := range i.dialogs {
		total += len(m.Messages)
	}
	return total
}

func (i *iter) ij2n(ii, jj int) int {
	return i.preSum[ii] + jj
}

func flatDialogs(dialogs [][]*tmessage.Dialog) []*tmessage.Dialog {
	res := make([]*tmessage.Dialog, 0)
	for _, d := range dialogs {
		if len(d) == 0 {
			continue
		}
		res = append(res, d...)
	}
	return res
}

func filterMap(data []string, keyFn func(key string) string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, v := range data {
		m[keyFn(v)] = struct{}{}
	}
	return m
}

func sortDialogs(dialogs []*tmessage.Dialog, desc bool) {
	sort.Slice(dialogs, func(i, j int) bool {
		return utils.Telegram.GetInputPeerID(dialogs[i].Peer) <
			utils.Telegram.GetInputPeerID(dialogs[j].Peer) // increasing order
	})

	for _, m := range dialogs {
		sort.Slice(m.Messages, func(i, j int) bool {
			if desc {
				return m.Messages[i] > m.Messages[j]
			}
			return m.Messages[i] < m.Messages[j]
		})
	}
}

// preSum of dialogs
func preSum(dialogs []*tmessage.Dialog) []int {
	sum := make([]int, len(dialogs)+1)
	for i, m := range dialogs {
		sum[i+1] = sum[i] + len(m.Messages)
	}
	return sum
}

func fingerprint(dialogs []*tmessage.Dialog) string {
	endian := binary.BigEndian
	buf, b := &bytes.Buffer{}, make([]byte, 8)
	for _, m := range dialogs {
		endian.PutUint64(b, uint64(utils.Telegram.GetInputPeerID(m.Peer)))
		buf.Write(b)
		for _, msg := range m.Messages {
			endian.PutUint64(b, uint64(msg))
			buf.Write(b)
		}
	}

	return fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
}

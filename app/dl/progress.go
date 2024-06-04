package dl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	pw "github.com/jedib0t/go-pretty/v6/progress"

	"github.com/iyear/tdl/pkg/downloader"
	"github.com/iyear/tdl/pkg/prog"
	"github.com/iyear/tdl/pkg/utils"
)

type progress struct {
	pw       pw.Writer
	trackers *sync.Map // map[ID]*pw.Tracker
	opts     Options

	it *iter
}

func newProgress(p pw.Writer, it *iter, opts Options) *progress {
	return &progress{
		pw:       p,
		trackers: &sync.Map{},
		opts:     opts,
		it:       it,
	}
}

func (p *progress) OnAdd(elem downloader.Elem) {
	tracker := prog.AppendTracker(p.pw, utils.Byte.FormatBinaryBytes, p.processMessage(elem), elem.File().Size())
	p.trackers.Store(elem.(*iterElem).id, tracker)
}

func (p *progress) OnDownload(elem downloader.Elem, state downloader.ProgressState) {
	tracker, ok := p.trackers.Load(elem.(*iterElem).id)
	if !ok {
		return
	}

	t := tracker.(*pw.Tracker)
	t.UpdateTotal(state.Total)
	t.SetValue(state.Downloaded)
}

func (p *progress) OnDone(elem downloader.Elem, err error, dirName string) {
	e := elem.(*iterElem)

	tracker, ok := p.trackers.Load(e.id)
	if !ok {
		return
	}
	t := tracker.(*pw.Tracker)

	if err := e.to.Close(); err != nil {
		p.fail(t, elem, errors.Wrap(err, "close file"))
		return
	}

	if err != nil {
		if !errors.Is(err, context.Canceled) { // don't report user cancel
			p.fail(t, elem, errors.Wrap(err, "progress"))
		}
		_ = os.Remove(e.to.Name()) // just try to remove temp file, ignore error
		return
	}

	p.it.Finish(e.id)

	if err := p.donePost(e, dirName); err != nil {
		p.fail(t, elem, errors.Wrap(err, "post file"))
		return
	}
}

func (p *progress) donePost(elem *iterElem, dirName string) (err error) {
	newfile := strings.TrimSuffix(filepath.Base(elem.to.Name()), tempExt)

	if p.opts.RewriteExt {
		mime, err := mimetype.DetectFile(elem.to.Name())
		if err != nil {
			return errors.Wrap(err, "detect mime")
		}
		ext := mime.Extension()
		if ext != "" && (filepath.Ext(newfile) != ext) {
			newfile = utils.FS.GetNameWithoutExt(newfile) + ext
		}
	}

	// create message dir
	subDirPath := fmt.Sprintf("%v/%v/%v", elem.From().ID(), "original", dirName)
	dirPath := filepath.Join(filepath.Dir(elem.to.Name()), subDirPath)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		fmt.Printf("Failed to create %v, error: %v\n", dirPath, err)
		return
	}

	// save message text
	text := elem.Msg().GetMessage()
	if text != "" {
		textPath := filepath.Join(dirPath, "info.txt")
		var textFile *os.File
		textFile, err = os.OpenFile(textPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			fmt.Printf("Failed to open %v, error: %v\n", textPath, err)
			return
		}
		_, err = textFile.WriteString(text)
		if err != nil {
			fmt.Printf("Failed to write to the record file, error: %v", err)
		}
	}

	// rename downloaded file
	if err := os.Rename(elem.to.Name(), filepath.Join(dirPath, newfile)); err != nil {
		return errors.Wrap(err, "rename file")
	}

	return nil
}

func (p *progress) fail(t *pw.Tracker, elem downloader.Elem, err error) {
	p.pw.Log(color.RedString("%s error: %s", p.elemString(elem), err.Error()))
	t.MarkAsErrored()
}

func (p *progress) processMessage(elem downloader.Elem) string {
	return p.elemString(elem)
}

func (p *progress) elemString(elem downloader.Elem) string {
	e := elem.(*iterElem)
	return fmt.Sprintf("%s(%d):%d -> %s",
		e.from.VisibleName(),
		e.from.ID(),
		e.fromMsg.ID,
		strings.TrimSuffix(e.to.Name(), tempExt))
}

package downloader

import (
	"context"
	"fmt"
	"github.com/gotd/td/tg"

	"github.com/go-faster/errors"
	"github.com/gotd/td/telegram/downloader"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/iyear/tdl/pkg/dcpool"
	"github.com/iyear/tdl/pkg/logger"
	"github.com/iyear/tdl/pkg/utils"
)

type Downloader struct {
	opts       Options
	dirNameMap map[int]string
}

type Options struct {
	Pool     dcpool.Pool
	PartSize int
	Threads  int
	Iter     Iter
	Progress Progress
}

func New(opts Options) *Downloader {
	return &Downloader{
		opts:       opts,
		dirNameMap: make(map[int]string),
	}
}

func (d *Downloader) Download(ctx context.Context, limit int) error {
	wg, wgctx := errgroup.WithContext(ctx)
	wg.SetLimit(limit)

	for d.opts.Iter.Next(wgctx) {
		elem := d.opts.Iter.Value()

		wg.Go(func() (rerr error) {
			d.opts.Progress.OnAdd(elem)

			defer func() {
				dirName, err := d.getDirName(elem, ctx)
				if err != nil {
					return
				}

				d.opts.Progress.OnDone(elem, rerr, dirName)

				if rerr == nil {
					d.opts.Iter.Record().Recorded("download", elem.From().ID(), elem.Msg().ID)
				}
			}()

			if err := d.download(wgctx, elem); err != nil {
				// canceled by user, so we directly return error to stop all
				if errors.Is(err, context.Canceled) {
					return errors.Wrap(err, "download")
				}

				// don't return error, just log it
			}

			return nil
		})
	}

	if err := d.opts.Iter.Err(); err != nil {
		return errors.Wrap(err, "iter")
	}

	return wg.Wait()
}

func (d *Downloader) download(ctx context.Context, elem Elem) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	logger.From(ctx).Debug("Start download elem",
		zap.Any("elem", elem))

	client := d.opts.Pool.Client(ctx, elem.File().DC())
	if elem.AsTakeout() {
		client = d.opts.Pool.Takeout(ctx, elem.File().DC())
	}

	_, err := downloader.NewDownloader().WithPartSize(d.opts.PartSize).
		Download(client, elem.File().Location()).
		WithThreads(utils.Telegram.BestThreads(elem.File().Size(), d.opts.Threads)).
		Parallel(ctx, newWriteAt(elem, d.opts.Progress, d.opts.PartSize))
	if err != nil {
		return errors.Wrap(err, "download")
	}

	return nil
}

func (d *Downloader) getDirName(elem Elem, ctx context.Context) (name string, err error) {
	if dirName, ok := d.dirNameMap[elem.Msg().ID]; ok {
		return dirName, nil
	} else {
		if _, ok := elem.Msg().GetGroupedID(); ok {
			var grouped []*tg.Message
			grouped, err = utils.Telegram.GetGroupedMessages(ctx, d.opts.Pool.Default(ctx), elem.From().InputPeer(), elem.Msg())
			if err != nil {
				fmt.Printf("Failed to get grouped messages, error: %v\n", err)
				return
			}

			var _min int = 0x7fffffff
			var _max int = -1
			for _, m := range grouped {
				if m.ID > _max {
					_max = m.ID
				}
				if m.ID < _min {
					_min = m.ID
				}
			}

			if _min == 0x7fffffff || _max == -1 {
				err = fmt.Errorf("Failed to sort grouped messages.")
				fmt.Printf("%v\n", err)
				return
			}
			name = fmt.Sprintf("%v-%v", _min, _max)

			for _, m := range grouped {
				d.dirNameMap[m.ID] = name
			}
		} else {
			name = fmt.Sprintf("%v", elem.Msg().ID)
		}
	}
	return
}

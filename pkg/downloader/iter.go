package downloader

import (
	"context"
	"github.com/gotd/td/telegram/peers"
	"github.com/iyear/tdl/pkg/iyzyi"
	"io"

	"github.com/gotd/td/tg"
)

type Iter interface {
	Next(ctx context.Context) bool
	Value() Elem
	Err() error
	Record() *iyzyi.Recorder
}

type Elem interface {
	File() File
	To() io.WriterAt

	AsTakeout() bool

	From() peers.Peer
	Msg() *tg.Message
}

type File interface {
	Location() tg.InputFileLocationClass
	Size() int64
	DC() int
}

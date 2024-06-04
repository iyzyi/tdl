package forwarder

import (
	"context"
	"github.com/iyear/tdl/pkg/iyzyi"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
)

type Iter interface {
	Next(ctx context.Context) bool
	Value() Elem
	Err() error
	Record() *iyzyi.Recorder
}

type Elem interface {
	Mode() Mode

	From() peers.Peer
	Msg() *tg.Message
	To() peers.Peer
	Thread() int // reply to message/topic

	AsSilent() bool
	AsDryRun() bool
	AsGrouped() bool // detect and forward grouped messages
}

package iyzyi

import (
	"context"
	"fmt"
	"github.com/gotd/td/telegram/peers"
	"github.com/iyear/tdl/pkg/tmessage"
)

func RemoveRecordedMessages(_type string, ctx context.Context, manager *peers.Manager, record *Recorder, dialogs *[]*tmessage.Dialog) (skip bool, err error) {
	for _, dialog := range *dialogs {
		var from peers.Peer
		from, err = manager.FromInputPeer(ctx, dialog.Peer)
		if err != nil {
			fmt.Printf("Failed to resolve from input peer, error: %v\n", err)
			return
		}
		fromID := from.ID()

		i := 0
		for _, msgID := range dialog.Messages {
			if !record.IsRecorded(_type, fromID, msgID) {
				dialog.Messages[i] = msgID
				i++
			} else {
				skip = true
				fmt.Printf("Skip MsgID %v as it has already been %sed\n", msgID, _type)
			}
		}
		dialog.Messages = dialog.Messages[:i]
	}

	i := 0
	for _, dialog := range *dialogs {
		if len(dialog.Messages) > 0 {
			(*dialogs)[i] = dialog
			i++
		}
	}
	*dialogs = (*dialogs)[:i]
	return
}

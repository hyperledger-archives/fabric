package shim

import (
	"fmt"

	"github.com/looplab/fsm"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	pb "github.com/openblockchain/obc-peer/protos"
)

// Handler handler implementation for shim side of chaincode
type Handler struct {
	To         string
	ChatStream chaincode.PeerChaincodeStream
	FSM        *fsm.FSM
}

// newChaincodeHandler return a new instance of the shim side handler
func newChaincodeHandler(to string, peerChatStream chaincode.PeerChaincodeStream) *Handler {
	v := &Handler{
		To:         to,
		ChatStream: peerChatStream,
	}

	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.ChaincodeMessage_REGISTERED.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(), Src: []string{"established"}, Dst: "established"},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTERED.String(): func(e *fsm.Event) { v.beforeRegistered(e) },
		},
	)
	return v
}

func (c *Handler) beforeRegistered(e *fsm.Event) {
	chaincodeLogger.Debug("Received  back %s, ready for invocations", pb.ChaincodeMessage_REGISTERED)
	if _, ok := e.Args[0].(*pb.ChaincodeMessage); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	//msg := e.Args[0].(*pb.ChaincodeMessage)
	//chaincodeLogger.Debug("Received %s, going to handle", ...)
	// if err := c.ChatStream.Send(&pb.ChaincodeMessage{Type: pb.OpenchainMessage_DISC_HELLO}); err != nil {
	// 	e.Cancel(err)
	// }
}

// HandleMessage message handling loop for shim side of chaincode/Peer stream
func (c *Handler) HandleMessage(msg *pb.ChaincodeMessage) error {
	chaincodeLogger.Debug("Handling ChaincodeMessage of type: %s ", msg.Type)
	if c.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Chaincode handler FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), c.FSM.Current())
	}
	err := c.FSM.Event(msg.Type.String(), msg)
	return filterError(err)
}

// Filter the Errors to allow NoTransitionError and CanceledError to not propogate for cases where embedded Err == nil
func filterError(errFromFSMEvent error) error {
	if errFromFSMEvent != nil {
		if noTransitionErr, ok := errFromFSMEvent.(*fsm.NoTransitionError); ok {
			if noTransitionErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return errFromFSMEvent
				//t.Error("expected only 'NoTransitionError'")
			}
			chaincodeLogger.Debug("Ignoring NoTransitionError: %s", noTransitionErr)
		}
		if canceledErr, ok := errFromFSMEvent.(*fsm.CanceledError); ok {
			if canceledErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return canceledErr
				//t.Error("expected only 'NoTransitionError'")
			}
			chaincodeLogger.Debug("Ignoring CanceledError: %s", canceledErr)
		}
	}
	return nil
}

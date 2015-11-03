package shim

import (
	"fmt"

	"github.com/looplab/fsm"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/consensus/pbft"
	pb "github.com/openblockchain/obc-peer/protos"
)

type MessageHandler interface {
	HandleMessage(msg *pb.ChaincodeMessage) error
	//SendMessage(msg *pb.ChaincodeMessage) error
}

type ChaincodeHandler struct {
	To         string
	ChatStream chaincode.PeerChaincodeStream
	FSM        *fsm.FSM
}

func NewChaincodeHandler(to string, peerChatStream chaincode.PeerChaincodeStream) *ChaincodeHandler {
	v := &ChaincodeHandler{
		To:         to,
		ChatStream: peerChatStream,
	}

	v.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.ChaincodeMessage_REGISTERED.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_CHAIN_TRANSACTIONS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pbft.PBFT_REQUEST.String(), Src: []string{"established"}, Dst: "prepare_result_sent"},
			{Name: pbft.PBFT_PRE_PREPARE.String(), Src: []string{"established"}, Dst: "prepare_result_sent"},
			{Name: pbft.PBFT_PREPARE_RESULT.String(), Src: []string{"prepare_result_sent"}, Dst: "commit_result_sent"},
			{Name: pbft.PBFT_COMMIT_RESULT.String(), Src: []string{"prepare_result_sent", "commit_result_sent"}, Dst: "committed_block"},
		},
		fsm.Callbacks{
			"before_" + pb.ChaincodeMessage_REGISTERED.String(): func(e *fsm.Event) { v.beforeRegistered(e) },
		},
	)
	return v
}

func (c *ChaincodeHandler) beforeRegistered(e *fsm.Event) {
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

func (c *ChaincodeHandler) HandleMessage(msg *pb.ChaincodeMessage) error {
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
			} else {
				chaincodeLogger.Debug("Ignoring NoTransitionError: %s", noTransitionErr)
			}
		}
		if canceledErr, ok := errFromFSMEvent.(*fsm.CanceledError); ok {
			if canceledErr.Err != nil {
				// Only allow NoTransitionError's, all others are considered true error.
				return canceledErr
				//t.Error("expected only 'NoTransitionError'")
			} else {
				chaincodeLogger.Debug("Ignoring CanceledError: %s", canceledErr)
			}
		}
	}
	return nil
}

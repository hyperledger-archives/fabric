/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package obcpbft

import (
	"github.com/hyperledger/fabric/consensus/obcpbft/events"
	pb "github.com/hyperledger/fabric/protos"
)

// --------------------------------------------------------------
//
// external contains all of the functions which
// are intended to be called from outside of the obcpbft package
//
// --------------------------------------------------------------

// Event types

// stateUpdatedEvent is sent when state transfer completes
type stateUpdatedEvent checkpointMessage

// stateUpdatingEvent is sent when state transfer is initiated
type stateUpdatingEvent checkpointMessage

type externalEventReceiver struct {
	manager events.Manager
}

// RecvMsg is called by the stack when a new message is received
func (eer *externalEventReceiver) RecvMsg(ocMsg *pb.Message, senderHandle *pb.PeerID) error {
	eer.manager.Queue() <- batchMessageEvent{
		msg:    ocMsg,
		sender: senderHandle,
	}
	return nil
}

// StateUpdated is a signal from the stack that it has fast-forwarded its state
func (eer *externalEventReceiver) StateUpdated(seqNo uint64, id []byte) {
	eer.manager.Queue() <- stateUpdatedEvent{
		seqNo: seqNo,
		id:    id,
	}
}

// StateUpdating is a signal from the stack that state transfer has started
func (eer *externalEventReceiver) StateUpdating(seqNo uint64, id []byte) {
	eer.manager.Queue() <- stateUpdatingEvent{
		seqNo: seqNo,
		id:    id,
	}
}

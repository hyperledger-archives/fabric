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

// --------------------------------------------------------
//
// legacyPbftShim is a temporary measure to allow the non-batch
// plugins to continue to function until they are completely
// deprecated
//
// --------------------------------------------------------

import (
	"fmt"

	"github.com/golang/protobuf/proto"
)

type legacyGenericShim struct {
	obcGeneric
	pbft legacyPbftShim
}

type legacyPbftShim struct {
	*pbftCore
}

// stateUpdated is an event telling us that the application fast-forwarded its state
func (instance legacyPbftShim) stateUpdated(seqNo uint64, id []byte) {
	logger.Debug("Replica %d queueing message that it has caught up via state transfer", instance.id)
	instance.manager.queue() <- stateUpdatedEvent{
		seqNo: seqNo,
		id:    id,
	}
}

// stateUpdating is an event telling us that the application is fast-forwarding its state
func (instance legacyPbftShim) stateUpdating(seqNo uint64, id []byte) {
	logger.Debug("Replica %d queueing message that state transfer has been initiated", instance.id)
	instance.manager.queue() <- stateUpdatingEvent{
		seqNo: seqNo,
		id:    id,
	}
}

// handle new consensus requests
func (instance legacyPbftShim) request(msgPayload []byte, senderID uint64) error {
	msg := &Message{&Message_Request{&Request{Payload: msgPayload,
		ReplicaId: senderID}}}
	instance.manager.queue() <- pbftMessageEvent{
		sender: senderID,
		msg:    msg,
	}
	return nil
}

// handle internal consensus messages
func (instance legacyPbftShim) receive(msgPayload []byte, senderID uint64) error {
	msg := &Message{}
	err := proto.Unmarshal(msgPayload, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	instance.manager.queue() <- pbftMessageEvent{
		msg:    msg,
		sender: senderID,
	}

	return nil
}

// TODO, this should not return an error
func (instance legacyPbftShim) recvMsgSync(msg *Message, senderID uint64) (err error) {
	instance.manager.queue() <- pbftMessageEvent{
		msg:    msg,
		sender: senderID,
	}
	return nil
}

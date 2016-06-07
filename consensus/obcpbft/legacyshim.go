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

	"github.com/hyperledger/fabric/consensus/obcpbft/events"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type legacyInnerStack interface {
	innerStack
	viewChange(curView uint64)
}

type legacyGenericShim struct {
	*obcGeneric
	pbft *legacyPbftShim
}

type legacyPbftShim struct {
	*pbftCore
	consumer legacyInnerStack
	manager  events.Manager // Used to give the pbft core work
}

func (shim *legacyGenericShim) init(id uint64, config *viper.Viper, consumer legacyInnerStack) {
	shim.pbft = &legacyPbftShim{
		manager:  events.NewManagerImpl(),
		consumer: consumer,
	}
	shim.pbft.pbftCore = newPbftCore(id, config, consumer, events.NewTimerFactoryImpl(shim.pbft.manager))
	shim.pbft.manager.SetReceiver(shim.pbft)
	logger.Debug("Replica %d Consumer is %p", shim.pbft.id, shim.pbft.consumer)
	shim.pbft.manager.Start()
	logger.Debug("Replica %d legacyGenericShim now initialized: %v", id, shim)
}

// Close releases the resources created by newLegacyGenericShim
func (shim *legacyGenericShim) Close() {
	shim.pbft.manager.Halt()
	shim.pbft.pbftCore.close()
}

// TODO, temporary measure until mock network gets more sophisticated
func (shim *legacyGenericShim) getManager() events.Manager {
	return shim.pbft.manager
}

// ProcessEvent intercepts the events bound for PBFT to implement the legacy innerStack methods
func (instance *legacyPbftShim) ProcessEvent(e events.Event) events.Event {
	switch e.(type) {
	case viewChangedEvent:
		instance.consumer.viewChange(instance.view)
	default:
		return instance.pbftCore.ProcessEvent(e)
	}
	return nil
}

// execDone is an event telling us that the last execution has completed
func (instance *legacyPbftShim) execDone() {
	instance.manager.Queue() <- execDoneEvent{}
}

// stateUpdated is an event telling us that the application fast-forwarded its state
func (instance *legacyPbftShim) stateUpdated(seqNo uint64, id []byte) {
	logger.Debug("Replica %d queueing message that it has caught up via state transfer", instance.id)
	instance.manager.Queue() <- stateUpdatedEvent{
		seqNo: seqNo,
		id:    id,
	}
}

// stateUpdating is an event telling us that the application is fast-forwarding its state
func (instance *legacyPbftShim) stateUpdating(seqNo uint64, id []byte) {
	logger.Debug("Replica %d queueing message that state transfer has been initiated", instance.id)
	instance.manager.Queue() <- stateUpdatingEvent{
		seqNo: seqNo,
		id:    id,
	}
}

// handle new consensus requests
func (instance *legacyPbftShim) request(msgPayload []byte, senderID uint64) error {
	msg := &Message{&Message_Request{&Request{Payload: msgPayload,
		ReplicaId: senderID}}}
	instance.manager.Queue() <- pbftMessageEvent{
		sender: senderID,
		msg:    msg,
	}
	return nil
}

// handle internal consensus messages
func (instance *legacyPbftShim) receive(msgPayload []byte, senderID uint64) error {
	msg := &Message{}
	err := proto.Unmarshal(msgPayload, msg)
	if err != nil {
		return fmt.Errorf("Error unpacking payload from message: %s", err)
	}

	instance.manager.Queue() <- pbftMessageEvent{
		msg:    msg,
		sender: senderID,
	}

	return nil
}

// TODO, this should not return an error
func (instance *legacyPbftShim) recvMsgSync(msg *Message, senderID uint64) (err error) {
	instance.manager.Queue() <- pbftMessageEvent{
		msg:    msg,
		sender: senderID,
	}
	return nil
}

// Allows the caller to inject work onto the main thread
// This is useful when the caller wants to safely manipulate PBFT state
func (instance *legacyPbftShim) inject(work func()) {
	instance.manager.Queue() <- workEvent(work)
}

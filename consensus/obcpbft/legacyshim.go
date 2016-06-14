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

	pb "github.com/hyperledger/fabric/protos"

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
	closed   chan struct{}
	consumer legacyInnerStack
	manager  events.Manager // Used to give the pbft core work
}

func (shim *legacyGenericShim) init(id uint64, config *viper.Viper, consumer legacyInnerStack) {
	shim.pbft = &legacyPbftShim{
		closed:   make(chan struct{}),
		manager:  events.NewManagerImpl(),
		consumer: consumer,
	}
	shim.pbft.pbftCore = newPbftCore(id, config, consumer, events.NewTimerFactoryImpl(shim.pbft.manager))
	shim.pbft.manager.SetReceiver(shim.pbft)
	logger.Debug("Replica %d Consumer is %p", shim.pbft.id, shim.pbft.consumer)
	shim.pbft.manager.Start()
	logger.Debug("Replica %d legacyGenericShim now initialized: %v", id, shim)
}

// Executed is called whenever Execute completes, no-op for now as the legacy code uses the legacy API
func (shim *legacyGenericShim) Executed(tag interface{}) {
	// Never called
}

// Committed is called whenever Commit completes, no-op for now as the legacy code uses the legacy API
func (shim *legacyGenericShim) Committed(tag interface{}, target *pb.BlockchainInfo) {
	// Never called
}

// RolledBack is called whenever a Rollback completes, no-op for now as the legacy code uses the legacy API
func (shim *legacyGenericShim) RolledBack(tag interface{}) {
	// Never called
}

// StatedUpdates is called when state transfer completes, if target is nil, this indicates a failure and a new target should be supplied, no-op for now as the legacy code uses the legacy API
func (shim *legacyGenericShim) StateUpdated(tag interface{}, target *pb.BlockchainInfo) {
	id, _ := proto.Marshal(target)
	chkpt := tag.(*checkpointMessage)

	shim.pbft.stateUpdated(chkpt.seqNo, id)
}

// Close releases the resources created by newLegacyGenericShim
func (shim *legacyGenericShim) Close() {
	select {
	case <-shim.pbft.closed:
	default:
		close(shim.pbft.closed)
	}
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
	logger.Debugf("Replica %d queueing message that it has caught up via state transfer", instance.id)
	info := &pb.BlockchainInfo{}
	err := proto.Unmarshal(id, info)
	if err != nil {
		logger.Errorf("Error unmarshaling: %s", err)
		return
	}
	instance.manager.Queue() <- stateUpdatedEvent{
		chkpt: &checkpointMessage{
			seqNo: seqNo,
			id:    id,
		},
		target: info,
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

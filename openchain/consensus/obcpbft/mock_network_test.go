/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package obcpbft

import (
	"fmt"

	pb "github.com/openblockchain/obc-peer/protos"
)

type endpoint interface {
	stop()
	idleChan() <-chan struct{}
	deliver([]byte, *pb.PeerID)
	getHandle() *pb.PeerID
}

type taggedMsg struct {
	src int
	dst int
	msg []byte
}

type testnet struct {
	N         int
	closed    bool
	endpoints []endpoint
	msgs      chan taggedMsg
	filterFn  func(int, int, []byte) []byte
}

type testEndpoint struct {
	id  uint64
	net *testnet
}

func makeTestEndpoint(id uint64, net *testnet) *testEndpoint {
	ep := &testEndpoint{}
	ep.id = id
	ep.net = net
	return ep
}

func (ep *testEndpoint) getHandle() *pb.PeerID {
	return &pb.PeerID{fmt.Sprintf("vp%d", ep.id)}
}

func (ep *testEndpoint) GetNetworkInfo() (self *pb.PeerEndpoint, network []*pb.PeerEndpoint, err error) {
	oSelf, oNetwork, _ := ep.GetNetworkHandles()
	self = &pb.PeerEndpoint{
		ID:   oSelf,
		Type: pb.PeerEndpoint_VALIDATOR,
	}

	network = make([]*pb.PeerEndpoint, len(oNetwork))
	for i, id := range oNetwork {
		network[i] = &pb.PeerEndpoint{
			ID:   id,
			Type: pb.PeerEndpoint_VALIDATOR,
		}
	}
	return
}

func (ep *testEndpoint) GetNetworkHandles() (self *pb.PeerID, network []*pb.PeerID, err error) {
	if nil == ep.net {
		err = fmt.Errorf("Network not initialized")
		return
	}
	self = ep.getHandle()
	network = make([]*pb.PeerID, len(ep.net.endpoints))
	for i, oep := range ep.net.endpoints {
		if nil != oep {
			// In case this is invoked before all endpoints are initialized, this emulates a real network as well
			network[i] = oep.getHandle()
		}
	}
	return
}

// Broadcast delivers to all endpoints.  In contrast to the stack
// Broadcast, this will also deliver back to the replica.  We keep
// this behavior, because it exposes subtle bugs in the
// implementation.
func (ep *testEndpoint) Broadcast(msg *pb.OpenchainMessage, peerType pb.PeerEndpoint_Type) error {
	ep.net.broadcastFilter(ep, msg.Payload)
	return nil
}

func (ep *testEndpoint) Unicast(msg *pb.OpenchainMessage, receiverHandle *pb.PeerID) error {
	receiverID, err := getValidatorID(receiverHandle)
	if err != nil {
		return fmt.Errorf("Couldn't unicast message to %s: %v", receiverHandle.Name, err)
	}
	internalQueueMessage(ep.net.msgs, taggedMsg{int(ep.id), int(receiverID), msg.Payload})
	return nil
}

func internalQueueMessage(queue chan<- taggedMsg, tm taggedMsg) {
	select {
	case queue <- tm:
	default:
		logger.Warning("TEST NET: Message cannot be queued without blocking, consider increasing the queue size")
		queue <- tm
	}
}

func (net *testnet) broadcastFilter(ep *testEndpoint, payload []byte) {
	if net.closed {
		logger.Error("WARNING! Attempted to send a request to a closed network, ignoring")
		return
	}
	if net.filterFn != nil {
		tmp := payload
		payload = net.filterFn(int(ep.id), -1, payload)
		logger.Debug("TEST: filtered message %p to %p", tmp, payload)
	}
	if payload != nil {
		logger.Debug("TEST: attempting to queue message %p", payload)
		internalQueueMessage(net.msgs, taggedMsg{int(ep.id), -1, payload})
		logger.Debug("TEST: message queued successfully %p", payload)
	}
}

func (net *testnet) deliverFilter(msg taggedMsg, senderID int) {
	senderHandle := net.endpoints[senderID].getHandle()
	if msg.dst == -1 {
		for id, inst := range net.endpoints {
			if msg.src == id {
				// do not deliver to local replica
				continue
			}
			payload := msg.msg
			if net.filterFn != nil {
				payload = net.filterFn(msg.src, id, payload)
			}
			if payload != nil {
				inst.deliver(msg.msg, senderHandle)
			}
		}
	} else {
		net.endpoints[msg.dst].deliver(msg.msg, senderHandle)
	}
}

func (net *testnet) idleFan() <-chan struct{} {
	res := make(chan struct{})

	go func() {
		for _, inst := range net.endpoints {
			<-inst.idleChan()
		}
		logger.Debug("TEST: closing idleChan")
		// Only close to the channel after all the consenters have written to us
		close(res)
	}()

	return res
}

func (net *testnet) processMessageFromChannel(msg taggedMsg, ok bool) bool {
	if !ok {
		logger.Debug("TEST: message channel closed, exiting\n")
		return false
	}
	logger.Debug("TEST: new message, delivering\n")
	net.deliverFilter(msg, msg.src)
	return true
}

func (net *testnet) process() error {
	for {
		logger.Debug("TEST: process looping")
		select {
		case msg, ok := <-net.msgs:
			logger.Debug("TEST: processing message without testing for idle")
			if !net.processMessageFromChannel(msg, ok) {
				return nil
			}
		default:
			logger.Debug("TEST: processing message or testing for idle")
			select {
			case <-net.idleFan():
				logger.Debug("TEST: exiting process loop because of idleness")
				return nil
			case msg, ok := <-net.msgs:
				if !net.processMessageFromChannel(msg, ok) {
					return nil
				}
			}
		}
	}

	return nil
}

func (net *testnet) processContinually() {
	for {
		msg, ok := <-net.msgs
		if !net.processMessageFromChannel(msg, ok) {
			return
		}
	}
}

func makeTestnet(N int, initFn func(id uint64, network *testnet) endpoint) *testnet {
	net := &testnet{}
	net.msgs = make(chan taggedMsg, 100)
	net.endpoints = make([]endpoint, N)

	for i, _ := range net.endpoints {
		net.endpoints[i] = initFn(uint64(i), net)
	}

	return net
}

func (net *testnet) clearMessages() {
	for {
		select {
		case <-net.msgs:
		default:
			return
		}
	}
}

func (net *testnet) stop() {
	net.closed = true
	close(net.msgs)
	for _, ep := range net.endpoints {
		ep.stop()
	}
}

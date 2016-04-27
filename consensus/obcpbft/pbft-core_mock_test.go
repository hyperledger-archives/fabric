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

	pb "github.com/hyperledger/fabric/protos"
)

type pbftEndpoint struct {
	*testEndpoint
	pbft *pbftCore
	sc   *simpleConsumer
}

func (pe *pbftEndpoint) deliver(msg []byte, senderHandle *pb.PeerID) {
	senderID, _ := getValidatorID(senderHandle)
	pe.pbft.receive(msg, senderID)
}

func (pe *pbftEndpoint) stop() {
	pe.pbft.close()
}

func (pe *pbftEndpoint) isBusy() bool {
	if pe.pbft.timerActive || pe.pbft.currentExec != nil {
		pe.net.debugMsg("TEST: Returning as busy because timer active (%v) or current exec (%v)\n", pe.pbft.timerActive, pe.pbft.currentExec)
		return true
	}

	// TODO, this looks racey, but seems fine, because the message send is on an unbuffered
	// channel, the send blocks until the thread has picked up the new work, still
	// this will be removed pending the transition to an externally driven state machine
	select {
	case <-pe.pbft.idleChan:
	default:
		pe.net.debugMsg("TEST: Returning as busy no reply on idleChan\n")
		return true
	}

	return false
}

type pbftNetwork struct {
	*testnet
	pbftEndpoints []*pbftEndpoint
}

type simpleConsumer struct {
	pe            *pbftEndpoint
	pbftNet       *pbftNetwork
	executions    uint64
	lastSeqNo     uint64
	skipOccurred  bool
	lastExecution []byte
	mockPersist
}

func (sc *simpleConsumer) broadcast(msgPayload []byte) {
	sc.pe.Broadcast(&pb.Message{Payload: msgPayload}, pb.PeerEndpoint_VALIDATOR)
}
func (sc *simpleConsumer) unicast(msgPayload []byte, receiverID uint64) error {
	handle, err := getValidatorHandle(receiverID)
	if nil != err {
		return err
	}
	sc.pe.Unicast(&pb.Message{Payload: msgPayload}, handle)
	return nil
}

func (sc *simpleConsumer) Close() {
	// No-op
}

func (sc *simpleConsumer) validate(txRaw []byte) error {
	return nil
}

func (sc *simpleConsumer) sign(msg []byte) ([]byte, error) {
	return msg, nil
}

func (sc *simpleConsumer) verify(senderID uint64, signature []byte, message []byte) error {
	return nil
}

func (sc *simpleConsumer) viewChange(curView uint64) {
}

func (sc *simpleConsumer) skipTo(seqNo uint64, id []byte, replicas []uint64) {
	sc.skipOccurred = true
	sc.executions = seqNo
	sc.pbftNet.debugMsg("TEST: skipping to %d\n", seqNo)
}

func (sc *simpleConsumer) execute(seqNo uint64, tx []byte) {
	sc.pbftNet.debugMsg("TEST: executing request\n")
	sc.lastExecution = tx
	sc.executions++
	sc.lastSeqNo = seqNo
	go sc.pe.pbft.execDone()
}

func (sc *simpleConsumer) getState() []byte {
	return []byte(fmt.Sprintf("%d", sc.executions))
}

func (sc *simpleConsumer) getLastSeqNo() (uint64, error) {
	if sc.executions < 1 {
		return 0, fmt.Errorf("no execution yet")
	}
	return sc.lastSeqNo, nil
}

func makePBFTNetwork(N int, initFNs ...func(pe *pbftEndpoint)) *pbftNetwork {

	endpointFunc := func(id uint64, net *testnet) endpoint {
		tep := makeTestEndpoint(id, net)
		pe := &pbftEndpoint{
			testEndpoint: tep,
		}

		pe.sc = &simpleConsumer{
			pe: pe,
		}

		pe.pbft = newPbftCore(id, loadConfig(), pe.sc)
		pe.pbft.N = N
		pe.pbft.f = (N - 1) / 3

		for _, fn := range initFNs {
			fn(pe)
		}

		return pe

	}

	pn := &pbftNetwork{testnet: makeTestnet(N, endpointFunc)}
	pn.pbftEndpoints = make([]*pbftEndpoint, len(pn.endpoints))
	for i, ep := range pn.endpoints {
		pn.pbftEndpoints[i] = ep.(*pbftEndpoint)
		pn.pbftEndpoints[i].sc.pbftNet = pn
	}
	return pn
}

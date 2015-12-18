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
	gp "google/protobuf"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	pb "github.com/openblockchain/obc-peer/protos"
)

func makeTestnetPbftCore(inst *instance) {
	config := readConfig()
	inst.pbft = newPbftCore(uint64(inst.id), config, inst)
	inst.pbft.replicaCount = len(inst.net.replicas)
	inst.pbft.f = inst.net.f
	inst.deliver = func(msg []byte) { inst.pbft.receive(msg) }
}

func TestEnvOverride(t *testing.T) {
	config := readConfig()

	key := "general.name"                       // for a key that exists
	envName := "OPENCHAIN_OBCPBFT_GENERAL_NAME" // env override name
	overrideValue := "overide_test"             // value to override default value with

	// test key
	if ok := config.IsSet("general.name"); !ok {
		t.Fatalf("Cannot test env override because \"%s\" does not seem to be set", key)
	}

	os.Setenv(envName, overrideValue)
	// The override config value will cause other calls to fail unless unset.
	defer func() {
		os.Unsetenv(envName)
	}()

	if ok := config.IsSet("general.name"); !ok {
		t.Fatalf("Env override in place, and key \"%s\" is not set", key)
	}

	// read key
	configVal := config.GetString("general.name")
	if configVal != overrideValue {
		t.Fatalf("Env override in place, expected key \"%s\" to be \"%s\" but instead got \"%s\"", key, overrideValue, configVal)
	}

}

func TestMaliciousPrePrepare(t *testing.T) {
	mock := newMock()
	instance := newPbftCore(1, readConfig(), mock)
	defer instance.close()
	instance.replicaCount = 5

	digest1 := "hi there"
	request2 := &Request{Payload: []byte("other")}

	nestedMsg := &Message{&Message_PrePrepare{&PrePrepare{
		View:           0,
		SequenceNumber: 1,
		RequestDigest:  digest1,
		Request:        request2,
		ReplicaId:      0,
	}}}
	err := instance.recvMsgSync(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to handle PBFT message: %s", err)
	}

	if len(mock.broadcasted) != 0 {
		t.Fatalf("Expected to ignore malicious pre-prepare")
	}
}

func TestIncompletePayload(t *testing.T) {
	mock := newMock()
	instance := newPbftCore(1, readConfig(), mock)
	defer instance.close()
	instance.replicaCount = 5

	checkMsg := func(msg *Message, errMsg string, args ...interface{}) {
		_ = instance.recvMsgSync(msg)
		if len(mock.broadcasted) != 0 {
			t.Errorf(errMsg, args...)
		}
	}

	checkMsg(&Message{}, "Expected to reject empty message")
	checkMsg(&Message{&Message_Request{&Request{}}}, "Expected to reject empty request")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{}}}, "Expected to reject empty pre-prepare")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{SequenceNumber: 1}}}, "Expected to reject incomplete pre-prepare")
}

func TestNetwork(t *testing.T) {
	net := makeTestnet(2, makeTestnetPbftCore)
	defer net.close()

	// Create a message of type: `OpenchainMessage_CHAIN_TRANSACTION`
	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	err = net.replicas[0].pbft.request(txPacked)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if len(inst.executed) == 0 {
			t.Errorf("Instance %d did not execute transaction", inst.id)
			continue
		}
		if len(inst.executed) != 1 {
			t.Errorf("Instance %d executed more than one transaction", inst.id)
			continue
		}
		if !reflect.DeepEqual(inst.executed[0], txPacked) {
			t.Errorf("Instance %d executed wrong transaction, %s should be %s",
				inst.id, inst.executed[0], txPacked)
		}
	}
}

func TestCheckpoint(t *testing.T) {
	net := makeTestnet(1, func(inst *instance) {
		makeTestnetPbftCore(inst)
		inst.pbft.K = 2
	})
	defer net.close()

	execReq := func(iter int64) {
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}
		msg := &Message{&Message_Request{&Request{Payload: txPacked}}}
		err = net.replicas[0].pbft.recvMsgSync(msg)
		if err != nil {
			t.Fatalf("Request failed: %s", err)
		}

		err = net.process()
		if err != nil {
			t.Fatalf("Processing failed: %s", err)
		}
	}

	execReq(1)
	execReq(2)

	for _, inst := range net.replicas {
		if len(inst.pbft.chkpts) != 1 {
			t.Errorf("Expected 1 checkpoint, found %d", len(inst.pbft.chkpts))
			continue
		}

		if _, ok := inst.pbft.chkpts[2]; !ok {
			t.Errorf("Expected checkpoint for seqNo 2, got %s", inst.pbft.chkpts)
			continue
		}

		if inst.pbft.h != 2 {
			t.Errorf("Expected low water mark to be 2, got %d", inst.pbft.h)
			continue
		}
	}
}

func TestLostPrePrepare(t *testing.T) {
	net := makeTestnet(1, makeTestnetPbftCore)
	defer net.close()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	req := &Request{
		Timestamp: &gp.Timestamp{Seconds: 1, Nanos: 0},
		Payload:   txPacked,
	}

	_ = net.replicas[0].pbft.recvRequest(req)

	// clear all messages sent by primary
	msg := net.msgs[0]
	net.msgs = net.msgs[:0]

	// deliver pre-prepare to subset of replicas
	for _, inst := range net.replicas[1 : len(net.replicas)-1] {
		msgReq := &Message{}
		err := proto.Unmarshal(msg.msg, msgReq)
		if err != nil {
			t.Fatal("Could not unmarshal message")
		}
		inst.pbft.recvMsgSync(msgReq)
	}

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if inst.id != 3 && len(inst.executed) != 1 {
			t.Errorf("Expected execution")
			continue
		}
		if inst.id == 3 && len(inst.executed) != 0 {
			t.Errorf("Expected no execution")
			continue
		}
	}
}

func TestInconsistentPrePrepare(t *testing.T) {
	net := makeTestnet(1, makeTestnetPbftCore)
	defer net.close()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	makePP := func(iter int64) *PrePrepare {
		req := &Request{
			Timestamp: &gp.Timestamp{Seconds: iter, Nanos: 0},
			Payload:   txPacked,
		}
		preprep := &PrePrepare{
			View:           0,
			SequenceNumber: 1,
			RequestDigest:  hashReq(req),
			Request:        req,
			ReplicaId:      0,
		}
		return preprep
	}

	_ = net.replicas[0].pbft.recvRequest(makePP(1).Request)

	// clear all messages sent by primary
	net.msgs = net.msgs[:0]

	// replace with fake messages
	_ = net.replicas[1].pbft.recvPrePrepare(makePP(1))
	_ = net.replicas[2].pbft.recvPrePrepare(makePP(2))
	_ = net.replicas[3].pbft.recvPrePrepare(makePP(3))

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if len(inst.executed) != 0 {
			t.Errorf("Expected no execution")
			continue
		}
	}
}

func TestViewChange(t *testing.T) {
	net := makeTestnet(1, func(inst *instance) {
		makeTestnetPbftCore(inst)
		inst.pbft.K = 2
		inst.pbft.L = inst.pbft.K * 2
	})
	defer net.close()

	execReq := func(iter int64) {
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}
		msg := &Message{&Message_Request{&Request{Payload: txPacked}}}
		err = net.replicas[0].pbft.recvMsgSync(msg)
		if err != nil {
			t.Fatalf("Request failed: %s", err)
		}

		err = net.process()
		if err != nil {
			t.Fatalf("Processing failed: %s", err)
		}
	}

	execReq(1)
	execReq(2)
	execReq(3)

	for i := 2; i < len(net.replicas); i++ {
		net.replicas[i].pbft.sendViewChange()
	}

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	if net.replicas[1].pbft.view != 1 || net.replicas[0].pbft.view != 1 {
		t.Fatalf("Replicas did not follow f+1 crowd to trigger view-change")
	}

	cp, ok := net.replicas[1].pbft.selectInitialCheckpoint(net.replicas[1].pbft.getViewChanges())
	if !ok || cp != 2 {
		t.Fatalf("Wrong new initial checkpoint: %+v",
			net.replicas[1].pbft.viewChangeStore)
	}

	msgList := net.replicas[1].pbft.assignSequenceNumbers(net.replicas[1].pbft.getViewChanges(), cp)
	if msgList[4] != "" || msgList[5] != "" || msgList[3] == "" {
		t.Fatalf("Wrong message list: %+v", msgList)
	}
}

func TestInconsistentDataViewChange(t *testing.T) {
	net := makeTestnet(1, makeTestnetPbftCore)
	defer net.close()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	makePP := func(iter int64) *PrePrepare {
		req := &Request{
			Timestamp: &gp.Timestamp{Seconds: iter, Nanos: 0},
			Payload:   txPacked,
		}
		preprep := &PrePrepare{
			View:           0,
			SequenceNumber: 1,
			RequestDigest:  hashReq(req),
			Request:        req,
			ReplicaId:      0,
		}
		return preprep
	}

	_ = net.replicas[0].pbft.recvRequest(makePP(0).Request)

	// clear all messages sent by primary
	net.msgs = net.msgs[:0]

	// replace with fake messages
	_ = net.replicas[1].pbft.recvPrePrepare(makePP(1))
	_ = net.replicas[2].pbft.recvPrePrepare(makePP(1))
	_ = net.replicas[3].pbft.recvPrePrepare(makePP(0))

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if len(inst.executed) != 0 {
			t.Errorf("Expected no execution")
			continue
		}
	}

	for _, inst := range net.replicas {
		inst.pbft.sendViewChange()
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	// XXX once state transfer works, make sure that a request
	// was executed by all replicas.
}

func TestNewViewTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test")
	}

	net := makeTestnet(1, func(inst *instance) {
		makeTestnetPbftCore(inst)
		inst.pbft.newViewTimeout = 100 * time.Millisecond
		inst.pbft.requestTimeout = inst.pbft.newViewTimeout
		inst.pbft.lastNewViewTimeout = inst.pbft.newViewTimeout
	})
	defer net.close()

	replica1Disabled := false
	net.filterFn = func(src int, dst int, msg []byte) []byte {
		if dst == -1 && src == 1 && replica1Disabled {
			return nil
		}
		return msg
	}

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)
	msg := &Message{&Message_Request{&Request{Payload: txPacked}}}
	msgPacked, _ := proto.Marshal(msg)

	go net.processContinually()

	// This will eventually trigger 1's request timeout
	// We check that one single timed out replica will not keep trying to change views by itself
	net.replicas[1].pbft.receive(msgPacked)
	time.Sleep(1 * time.Second)

	// This will eventually trigger 3's request timeout, which will lead to a view change to 1.
	// However, we disable 1, which will disable the new-view going through.
	// This checks that replicas will automatically move to view 2 when the view change times out.
	// However, 2 does not know about the missing request, and therefore the request will not be
	// pre-prepared and finally executed.  This will lead to another view-change timeout, even on
	// the replicas that never saw the request (e.g. 0)
	// Finally, 3 will be new primary and pre-prepare the missing request.
	replica1Disabled = true
	net.replicas[3].pbft.receive(msgPacked)
	time.Sleep(1 * time.Second)

	net.close()
	for i, inst := range net.replicas {
		if inst.pbft.view != 3 {
			t.Fatalf("Should have reached view 3, got %d instead for replica %d", inst.pbft.view, i)
		}
	}
}

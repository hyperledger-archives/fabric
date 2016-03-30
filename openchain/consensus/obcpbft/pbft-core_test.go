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
	gp "google/protobuf"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"

	pb "github.com/openblockchain/obc-peer/protos"
)

func init() {
	logging.SetLevel(logging.DEBUG, "")
}

func TestEnvOverride(t *testing.T) {
	config := loadConfig()

	key := "general.mode"                       // for a key that exists
	envName := "OPENCHAIN_OBCPBFT_GENERAL_MODE" // env override name
	overrideValue := "overide_test"             // value to override default value with

	// test key
	if ok := config.IsSet("general.mode"); !ok {
		t.Fatalf("Cannot test env override because \"%s\" does not seem to be set", key)
	}

	os.Setenv(envName, overrideValue)
	// The override config value will cause other calls to fail unless unset.
	defer func() {
		os.Unsetenv(envName)
	}()

	if ok := config.IsSet("general.mode"); !ok {
		t.Fatalf("Env override in place, and key \"%s\" is not set", key)
	}

	// read key
	configVal := config.GetString("general.mode")
	if configVal != overrideValue {
		t.Fatalf("Env override in place, expected key \"%s\" to be \"%s\" but instead got \"%s\"", key, overrideValue, configVal)
	}

}

func TestMaliciousPrePrepare(t *testing.T) {
	mock := &omniProto{
		broadcastImpl: func(msgPayload []byte) {
			t.Fatalf("Expected to ignore malicious pre-prepare")
		},
	}
	instance := newPbftCore(1, loadConfig(), mock, []byte("GENESIS"))
	defer instance.close()
	instance.replicaCount = 5

	digest1 := "hi there"
	request2 := &Request{Payload: []byte("other"), ReplicaId: uint64(generateBroadcaster(instance.replicaCount))}

	pbftMsg := &Message{&Message_PrePrepare{&PrePrepare{
		View:           0,
		SequenceNumber: 1,
		RequestDigest:  digest1,
		Request:        request2,
		ReplicaId:      0,
	}}}
	err := instance.recvMsgSync(pbftMsg, 0)
	if err != nil {
		t.Fatalf("Failed to handle PBFT message: %s", err)
	}
}

func TestWrongReplicaID(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount)
	defer net.stop()

	chainTxMsg := createOcMsgWithChainTx(1)
	req := &Request{
		Timestamp: chainTxMsg.Timestamp,
		Payload:   chainTxMsg.Payload,
		ReplicaId: 1,
	}
	pbftMsg := &Message{&Message_Request{req}}
	err := net.pbftEndpoints[0].pbft.recvMsgSync(pbftMsg, 0)

	if err == nil {
		t.Fatalf("Shouldn't have processed message with incorrect replica ID")
	}

	if err != nil {
		rightError := strings.HasPrefix(err.Error(), "Sender ID")
		if !rightError {
			t.Fatalf("Should have returned error about incorrect replica ID on the incoming message")
		}
	}
}

func TestIncompletePayload(t *testing.T) {
	mock := &omniProto{
		validateImpl: func(msg []byte) error {
			return nil
		},
	}
	instance := newPbftCore(1, loadConfig(), mock, []byte("GENESIS"))
	defer instance.close()
	instance.replicaCount = 5

	broadcaster := uint64(generateBroadcaster(instance.replicaCount))

	checkMsg := func(msg *Message, errMsg string, args ...interface{}) {
		mock.broadcastImpl = func(msgPayload []byte) {
			t.Errorf(errMsg, args...)
		}
		_ = instance.recvMsgSync(msg, broadcaster)
	}

	checkMsg(&Message{}, "Expected to reject empty message")
	checkMsg(&Message{&Message_Request{&Request{ReplicaId: broadcaster}}}, "Expected to reject empty request")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{ReplicaId: broadcaster}}}, "Expected to reject empty pre-prepare")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{SequenceNumber: 1, ReplicaId: broadcaster}}}, "Expected to reject incomplete pre-prepare")
}

func TestNetwork(t *testing.T) {
	validatorCount := 7
	net := makePBFTNetwork(validatorCount)
	defer net.stop()

	msg := createOcMsgWithChainTx(1)
	err := net.pbftEndpoints[0].pbft.request(msg.Payload, uint64(generateBroadcaster(validatorCount)))
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, pep := range net.pbftEndpoints {
		if pep.sc.executions <= 0 {
			t.Errorf("Instance %d did not execute transaction", pep.id)
			continue
		}
		if pep.sc.executions != 1 {
			t.Errorf("Instance %d executed more than one transaction", pep.id)
			continue
		}
		if !reflect.DeepEqual(pep.sc.lastExecution, msg.Payload) {
			t.Errorf("Instance %d executed wrong transaction, %x should be %x",
				pep.id, pep.sc.lastExecution, msg.Payload)
		}
	}
}

type checkpointConsumer struct {
	simpleConsumer
	execWait *sync.WaitGroup
}

func (cc *checkpointConsumer) execute(seqNo uint64, tx []byte, execInfo *ExecutionInfo) {
	if execInfo.Checkpoint {
		// For this particular case, we don't want the simple blocking behavior of execute
	}
}

func TestCheckpoint(t *testing.T) {
	execWait := &sync.WaitGroup{}
	finishWait := &sync.WaitGroup{}

	validatorCount := 4
	net := makePBFTNetwork(validatorCount, func(pe *pbftEndpoint) {
		pe.pbft.K = 2
		pe.pbft.L = 4
		pe.sc.checkpointResult = func(seqNo uint64, id []byte) {
			finishWait.Add(1)
			go func() {
				fmt.Println("TEST: possibly delaying checkpoint evaluation")
				execWait.Wait()
				fmt.Println("TEST: sending checkpoint")
				pe.pbft.Checkpoint(seqNo, id)
				finishWait.Done()
			}()
		}
	})
	defer net.stop()

	execReq := func(iter int64) {
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}
		msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: uint64(generateBroadcaster(validatorCount))}}}
		net.pbftEndpoints[0].pbft.recvMsgSync(msg, msg.GetRequest().ReplicaId)

		net.process()
	}

	// execWait is 0, and execute will proceed
	execReq(1)
	execReq(2)
	finishWait.Wait()
	net.process()

	for _, pep := range net.pbftEndpoints {
		if len(pep.pbft.chkpts) != 1 {
			t.Errorf("Expected 1 checkpoint, found %d", len(pep.pbft.chkpts))
			continue
		}

		if _, ok := pep.pbft.chkpts[2]; !ok {
			t.Errorf("Expected checkpoint for seqNo 2")
			continue
		}

		if pep.pbft.h != 2 {
			t.Errorf("Expected low water mark to be 2, got %d", pep.pbft.h)
			continue
		}
	}

	// this will block executes for now
	execWait.Add(1)
	execReq(3)
	execReq(4)
	execReq(5)
	execReq(6)

	// by now the requests should have committed, but not yet executed
	// we also reached the high water mark by now.

	execReq(7)

	// request 7 should not have committed, because no more free seqNo
	// could be assigned.

	// unblock executes.
	execWait.Add(-1)

	net.process()
	finishWait.Wait() // Decoupling the execution thread makes this nastiness necessary
	net.process()

	// by now request 7 should have been confirmed and executed

	for _, pep := range net.pbftEndpoints {
		expectedExecutions := uint64(7)
		if pep.sc.executions != expectedExecutions {
			t.Errorf("Should have executed %d, got %d instead for replica %d", expectedExecutions, pep.sc.executions, pep.id)
		}
	}
}

func TestLostPrePrepare(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount)
	defer net.stop()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	req := &Request{
		Timestamp: &gp.Timestamp{Seconds: 1, Nanos: 0},
		Payload:   txPacked,
		ReplicaId: uint64(generateBroadcaster(validatorCount)),
	}

	_ = net.pbftEndpoints[0].pbft.recvRequest(req)

	// clear all messages sent by primary
	msg := <-net.msgs
	net.clearMessages()

	// deliver pre-prepare to subset of replicas
	for _, pep := range net.pbftEndpoints[1 : len(net.pbftEndpoints)-1] {
		msgReq := &Message{}
		err := proto.Unmarshal(msg.msg, msgReq)
		if err != nil {
			t.Fatal("Could not unmarshal message")
		}
		pep.pbft.recvMsgSync(msgReq, uint64(msg.src))
	}

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, pep := range net.pbftEndpoints {
		if pep.id != 3 && pep.sc.executions != 1 {
			t.Errorf("Expected execution on replica %d", pep.id)
			continue
		}
		if pep.id == 3 && pep.sc.executions > 0 {
			t.Errorf("Expected no execution")
			continue
		}
	}
}

func TestInconsistentPrePrepare(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount)
	defer net.stop()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	makePP := func(iter int64) *PrePrepare {
		req := &Request{
			Timestamp: &gp.Timestamp{Seconds: iter, Nanos: 0},
			Payload:   txPacked,
			ReplicaId: uint64(generateBroadcaster(validatorCount)),
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

	_ = net.pbftEndpoints[0].pbft.recvRequest(makePP(1).Request)

	// clear all messages sent by primary
	net.clearMessages()

	// replace with fake messages
	_ = net.pbftEndpoints[1].pbft.recvPrePrepare(makePP(1))
	_ = net.pbftEndpoints[2].pbft.recvPrePrepare(makePP(2))
	_ = net.pbftEndpoints[3].pbft.recvPrePrepare(makePP(3))

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, pep := range net.pbftEndpoints {
		if pep.sc.executions > 0 {
			t.Errorf("Expected no execution")
			continue
		}
	}
}

func TestViewChange(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount, func(pep *pbftEndpoint) {
		pep.pbft.K = 2
		pep.pbft.L = pep.pbft.K * 2
	})
	defer net.stop()

	execReq := func(iter int64) {
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}
		msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: uint64(generateBroadcaster(validatorCount))}}}
		err = net.pbftEndpoints[0].pbft.recvMsgSync(msg, msg.GetRequest().ReplicaId)
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

	for i := 2; i < len(net.pbftEndpoints); i++ {
		net.pbftEndpoints[i].pbft.sendViewChange()
	}

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	if net.pbftEndpoints[1].pbft.view != 1 || net.pbftEndpoints[0].pbft.view != 1 {
		t.Fatalf("Replicas did not follow f+1 crowd to trigger view-change")
	}

	cp, ok, _ := net.pbftEndpoints[1].pbft.selectInitialCheckpoint(net.pbftEndpoints[1].pbft.getViewChanges())
	if !ok || cp.SequenceNumber != 2 {
		t.Fatalf("Wrong new initial checkpoint: %+v",
			net.pbftEndpoints[1].pbft.viewChangeStore)
	}

	msgList := net.pbftEndpoints[1].pbft.assignSequenceNumbers(net.pbftEndpoints[1].pbft.getViewChanges(), cp.SequenceNumber)
	if msgList[4] != "" || msgList[5] != "" || msgList[3] == "" {
		t.Fatalf("Wrong message list: %+v", msgList)
	}
}

func TestInconsistentDataViewChange(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount)
	defer net.stop()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	makePP := func(iter int64) *PrePrepare {
		req := &Request{
			Timestamp: &gp.Timestamp{Seconds: iter, Nanos: 0},
			Payload:   txPacked,
			ReplicaId: uint64(generateBroadcaster(validatorCount)),
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

	_ = net.pbftEndpoints[0].pbft.recvRequest(makePP(0).Request)

	// clear all messages sent by primary
	net.clearMessages()

	// replace with fake messages
	_ = net.pbftEndpoints[1].pbft.recvPrePrepare(makePP(1))
	_ = net.pbftEndpoints[2].pbft.recvPrePrepare(makePP(1))
	_ = net.pbftEndpoints[3].pbft.recvPrePrepare(makePP(0))

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, pep := range net.pbftEndpoints {
		if pep.sc.executions > 0 {
			t.Errorf("Expected no execution")
			continue
		}
	}

	for _, pep := range net.pbftEndpoints {
		pep.pbft.sendViewChange()
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, pep := range net.pbftEndpoints {
		if pep.sc.executions < 1 {
			t.Errorf("Expected execution")
			continue
		}
	}
}

func TestViewChangeWithStateTransfer(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount)
	defer net.stop()

	var err error

	for _, pep := range net.pbftEndpoints {
		pep.pbft.K = 2
		pep.pbft.L = 4
	}

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	broadcaster := uint64(generateBroadcaster(validatorCount))

	makePP := func(iter int64) *PrePrepare {
		req := &Request{
			Timestamp: &gp.Timestamp{Seconds: iter, Nanos: 0},
			Payload:   txPacked,
			ReplicaId: broadcaster,
		}
		preprep := &PrePrepare{
			View:           0,
			SequenceNumber: uint64(iter),
			RequestDigest:  hashReq(req),
			Request:        req,
			ReplicaId:      0,
		}
		return preprep
	}

	// Have primary advance the sequence number past a checkpoint for replicas 0,1,2
	for i := int64(1); i <= 3; i++ {
		_ = net.pbftEndpoints[0].pbft.recvRequest(makePP(i).Request)

		// clear all messages sent by primary
		net.clearMessages()

		_ = net.pbftEndpoints[0].pbft.recvPrePrepare(makePP(i))
		_ = net.pbftEndpoints[1].pbft.recvPrePrepare(makePP(i))
		_ = net.pbftEndpoints[2].pbft.recvPrePrepare(makePP(i))

		err = net.process()
		if err != nil {
			t.Fatalf("Processing failed: %s", err)
		}
	}

	fmt.Println("Done with stage 1")

	// Have primary now deliberately skip a sequence number to deadlock until viewchange
	_ = net.pbftEndpoints[0].pbft.recvRequest(makePP(4).Request)

	// clear all messages sent by primary
	net.clearMessages()

	// replace with fake messages
	_ = net.pbftEndpoints[1].pbft.recvPrePrepare(makePP(5))
	_ = net.pbftEndpoints[2].pbft.recvPrePrepare(makePP(5))
	_ = net.pbftEndpoints[3].pbft.recvPrePrepare(makePP(4))

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	fmt.Println("Done with stage 2")

	for _, pep := range net.pbftEndpoints {
		pep.pbft.sendViewChange()
	}

	fmt.Println("Done with stage 3")
	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	fmt.Println("Done with stage 4")

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}
	fmt.Println("Done with stage 5")

	for _, pep := range net.pbftEndpoints {
		if pep.sc.executions != 4 {
			t.Errorf("Replica %d expected execution through seqNo 5, with one null execution for seqNo 3, got %d executions", pep.pbft.id, pep.sc.executions)
			continue
		}
	}
	fmt.Println("Done with stage 6")
}

func TestNewViewTimeout(t *testing.T) {
	millisUntilTimeout := time.Duration(100)

	if testing.Short() {
		t.Skip("Skipping timeout test")
	}

	validatorCount := 4
	net := makePBFTNetwork(validatorCount, func(pe *pbftEndpoint) {
		pe.pbft.newViewTimeout = millisUntilTimeout * time.Millisecond
		pe.pbft.requestTimeout = pe.pbft.newViewTimeout
		pe.pbft.lastNewViewTimeout = pe.pbft.newViewTimeout
	})
	defer net.stop()

	replica1Disabled := false
	net.filterFn = func(src int, dst int, msg []byte) []byte {
		if dst == -1 && src == 1 && replica1Disabled {
			return nil
		}
		return msg
	}

	broadcaster := uint64(generateBroadcaster(validatorCount))

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)
	msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: broadcaster}}}
	msgPacked, _ := proto.Marshal(msg)

	go net.processContinually()

	// This will eventually trigger 1's request timeout
	// We check that one single timed out replica will not keep trying to change views by itself
	net.pbftEndpoints[1].pbft.receive(msgPacked, broadcaster)
	fmt.Println("Debug: Sleeping 1")
	time.Sleep(5 * millisUntilTimeout * time.Millisecond)
	fmt.Println("Debug: Waking 1")

	// This will eventually trigger 3's request timeout, which will lead to a view change to 1.
	// However, we disable 1, which will disable the new-view going through.
	// This checks that replicas will automatically move to view 2 when the view change times out.
	// However, 2 does not know about the missing request, and therefore the request will not be
	// pre-prepared and finally executed.
	replica1Disabled = true
	net.pbftEndpoints[3].pbft.receive(msgPacked, broadcaster)
	fmt.Println("Debug: Sleeping 2")
	time.Sleep(5 * millisUntilTimeout * time.Millisecond)
	fmt.Println("Debug: Waking 2")

	// So far, we are in view 2, and replica 1 and 3 (who got the request) in view change to view 3.
	// Submitting the request to 0 will eventually trigger its view-change timeout, which will make
	// all replicas move to view 3 and finally process the request.
	net.pbftEndpoints[0].pbft.receive(msgPacked, broadcaster)
	fmt.Println("Debug: Sleeping 3")
	time.Sleep(5 * millisUntilTimeout * time.Millisecond)
	fmt.Println("Debug: Waking 3")

	for i, pep := range net.pbftEndpoints {
		if pep.pbft.view < 3 {
			t.Errorf("Should have reached view 3, got %d instead for replica %d", pep.pbft.view, i)
		}
		executionsExpected := uint64(1)
		if pep.sc.executions != executionsExpected {
			t.Errorf("Should have executed %d, got %d instead for replica %d", executionsExpected, pep.sc.executions, i)
		}
	}
}

func TestViewChangeUpdateSeqNo(t *testing.T) {
	millisUntilTimeout := 100 * time.Millisecond

	validatorCount := 4
	net := makePBFTNetwork(validatorCount, func(pe *pbftEndpoint) {
		pe.pbft.newViewTimeout = millisUntilTimeout * time.Millisecond
		pe.pbft.requestTimeout = pe.pbft.newViewTimeout
		pe.pbft.lastNewViewTimeout = pe.pbft.newViewTimeout
		pe.pbft.lastExec = 99
		pe.pbft.h = 99 / pe.pbft.K * pe.pbft.K
	})
	net.pbftEndpoints[0].pbft.seqNo = 99

	go net.processContinually()

	msg := createOcMsgWithChainTx(1)
	net.pbftEndpoints[0].pbft.request(msg.Payload, uint64(generateBroadcaster(validatorCount)))
	time.Sleep(5 * millisUntilTimeout)
	// Now we all have executed seqNo 100.  After triggering a
	// view change, the new primary should pick up right after
	// that.

	net.pbftEndpoints[0].pbft.sendViewChange()
	net.pbftEndpoints[1].pbft.sendViewChange()
	time.Sleep(5 * millisUntilTimeout)

	msg = createOcMsgWithChainTx(2)
	net.pbftEndpoints[1].pbft.request(msg.Payload, uint64(generateBroadcaster(validatorCount)))
	time.Sleep(5 * millisUntilTimeout)

	net.stop()
	for i, pep := range net.pbftEndpoints {
		if pep.pbft.view < 1 {
			t.Errorf("Should have reached view 3, got %d instead for replica %d", pep.pbft.view, i)
		}
		executionsExpected := uint64(2)
		if pep.sc.executions != executionsExpected {
			t.Errorf("Should have executed %d, got %d instead for replica %d", executionsExpected, pep.sc.executions, i)
		}
	}
}

// From issue #687
func TestWitnessCheckpointOutOfBounds(t *testing.T) {
	mock := &omniProto{}
	instance := newPbftCore(1, loadConfig(), mock, []byte("GENESIS"))
	instance.f = 1
	instance.K = 2
	instance.L = 4
	defer instance.close()

	instance.recvCheckpoint(&Checkpoint{
		SequenceNumber: 6,
		ReplicaId:      0,
	})

	instance.moveWatermarks(6)

	// This causes the list of high checkpoints to grow to be f+1
	// even though there are not f+1 checkpoints witnessed outside our range
	// historically, this caused an index out of bounds error
	instance.recvCheckpoint(&Checkpoint{
		SequenceNumber: 10,
		ReplicaId:      3,
	})
}

// From issue #687
func TestWitnessFallBehindMissingPrePrepare(t *testing.T) {
	mock := &omniProto{}
	instance := newPbftCore(1, loadConfig(), mock, []byte("GENESIS"))
	instance.f = 1
	instance.K = 2
	instance.L = 4
	defer instance.close()

	instance.recvCommit(&Commit{
		SequenceNumber: 2,
		ReplicaId:      0,
	})

	// Historically, the lack of prePrepare associated with the commit would cause
	// a nil pointer reference
	instance.moveWatermarks(6)
}

func TestFallBehind(t *testing.T) {
	validatorCount := 4
	net := makePBFTNetwork(validatorCount, func(pep *pbftEndpoint) {
		pep.pbft.K = 2
		pep.pbft.L = 2 * pep.pbft.K
	})
	defer net.stop()

	execReq := func(iter int64, skipThree bool) {
		// Create a message of type `OpenchainMessage_CHAIN_TRANSACTION`
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}

		msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: uint64(generateBroadcaster(validatorCount))}}}

		err = net.pbftEndpoints[0].pbft.recvMsgSync(msg, msg.GetRequest().ReplicaId)
		if err != nil {
			t.Fatalf("Request failed: %s", err)
		}

		if skipThree {
			// Send the request for consensus to everone but replica 3
			net.filterFn = func(src, replica int, msg []byte) []byte {
				if src != -1 && replica == 3 {
					return nil
				}

				return msg
			}
		} else {
			// Send the request for consensus to everone
			net.filterFn = nil
		}
		err = net.process()

		if err != nil {
			t.Fatalf("Processing failed: %s", err)
		}
	}

	pep := net.pbftEndpoints[3]
	pbft := pep.pbft
	// Send enough requests to get to a checkpoint quorum certificate with sequence number L+K
	execReq(1, true)
	for request := int64(2); uint64(request) <= pbft.L+pbft.K; request++ {
		execReq(request, false)
	}

	if !pbft.skipInProgress {
		t.Fatalf("Replica did not detect that it has fallen behind.")
	}

	if len(pbft.chkpts) != 0 {
		t.Fatalf("Expected no checkpoints, found %d", len(pbft.chkpts))
	}

	if pbft.h != pbft.L+pbft.K {
		t.Fatalf("Expected low water mark to be %d, got %d", pbft.L+pbft.K, pbft.h)
	}

	// Send enough requests to get to a weak checkpoint certificate certain with sequence number L+K*2
	for request := int64(pbft.L + pbft.K + 1); uint64(request) <= pbft.L+pbft.K*2; request++ {
		execReq(request, false)
	}

	if !pep.sc.skipOccurred {
		t.Fatalf("Request failed to kick off state transfer")
	}

	execReq(int64(pbft.L+pbft.K*2+1), false)

	if pep.sc.executions != pbft.L+pbft.K*2+1 {
		t.Fatalf("Replica did not begin participating normally after state transfer completed")
	}
}

func TestPbftF0(t *testing.T) {
	net := makePBFTNetwork(1)
	defer net.stop()

	// Create a message of type: `OpenchainMessage_CHAIN_TRANSACTION`
	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime, Payload: []byte("TestNetwork")}
	txPacked, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}

	pep0 := net.pbftEndpoints[0]

	err = pep0.pbft.request(txPacked, 0)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, pep := range net.pbftEndpoints {
		if pep.sc.executions < 1 {
			t.Errorf("Instance %d did not execute transaction", pep.id)
			continue
		}
		if pep.sc.executions >= 2 {
			t.Errorf("Instance %d executed more than one transaction", pep.id)
			continue
		}
		if !reflect.DeepEqual(pep.sc.lastExecution, txPacked) {
			t.Errorf("Instance %d executed wrong transaction, %x should be %x",
				pep.id, pep.sc.lastExecution, txPacked)
		}
	}
}

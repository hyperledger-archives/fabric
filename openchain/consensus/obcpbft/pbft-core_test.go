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
	"bytes"
	"encoding/base64"
	"fmt"
	gp "google/protobuf"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"
)

func init() {
	logging.SetLevel(logging.DEBUG, "")
}

func makeTestnetPbftCore(inst *instance) {
	os.Setenv("OPENCHAIN_OBCPBFT_GENERAL_N", fmt.Sprintf("%d", inst.net.N)) // TODO, a little hacky, but needed for state transfer not to get upset
	defer func() {
		os.Unsetenv("OPENCHAIN_OBCPBFT_GENERAL_N")
	}()
	config := loadConfig()
	inst.pbft = newPbftCore(uint64(inst.id), config, inst, inst)
	inst.pbft.replicaCount = inst.net.N
	inst.pbft.f = inst.net.f
	inst.deliver = func(msg []byte, senderHandle *pb.PeerID) {
		senderID, _ := getValidatorID(senderHandle)
		inst.pbft.receive(msg, senderID)
	}
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
	mock := newMock()
	instance := newPbftCore(1, loadConfig(), mock, mock)
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

	if len(mock.broadcasted) != 0 {
		t.Fatalf("Expected to ignore malicious pre-prepare")
	}
}

func TestWrongReplicaID(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetPbftCore)
	defer net.close()

	chainTxMsg := createOcMsgWithChainTx(1)
	req := &Request{
		Timestamp: chainTxMsg.Timestamp,
		Payload:   chainTxMsg.Payload,
		ReplicaId: 1,
	}
	pbftMsg := &Message{&Message_Request{req}}
	err := net.replicas[0].pbft.recvMsgSync(pbftMsg, 0)

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
	mock := newMock()
	instance := newPbftCore(1, loadConfig(), mock, mock)
	defer instance.close()
	instance.replicaCount = 5

	broadcaster := uint64(generateBroadcaster(instance.replicaCount))

	checkMsg := func(msg *Message, errMsg string, args ...interface{}) {
		_ = instance.recvMsgSync(msg, broadcaster)
		if len(mock.broadcasted) != 0 {
			t.Errorf(errMsg, args...)
		}
	}

	checkMsg(&Message{}, "Expected to reject empty message")
	checkMsg(&Message{&Message_Request{&Request{ReplicaId: broadcaster}}}, "Expected to reject empty request")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{ReplicaId: broadcaster}}}, "Expected to reject empty pre-prepare")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{SequenceNumber: 1, ReplicaId: broadcaster}}}, "Expected to reject incomplete pre-prepare")
}

func TestNetwork(t *testing.T) {
	validatorCount := 7
	net := makeTestnet(validatorCount, makeTestnetPbftCore)
	defer net.close()

	msg := createOcMsgWithChainTx(1)
	err := net.replicas[0].pbft.request(msg.Payload, uint64(generateBroadcaster(validatorCount)))
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if blockHeight <= 1 {
			t.Errorf("Instance %d did not execute transaction", inst.id)
			continue
		}
		if blockHeight != 2 {
			t.Errorf("Instance %d executed more than one transaction", inst.id)
			continue
		}
		highestBlock, _ := inst.ledger.GetBlock(blockHeight - 1)
		if !reflect.DeepEqual(highestBlock.Transactions[0].Payload, msg.Payload) {
			t.Errorf("Instance %d executed wrong transaction, %x should be %x",
				inst.id, highestBlock.Transactions[0].Payload, msg.Payload)
		}
	}
}

func TestCheckpoint(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, func(inst *instance) {
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
		msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: uint64(generateBroadcaster(validatorCount))}}}
		err = net.replicas[0].pbft.recvMsgSync(msg, msg.GetRequest().ReplicaId)
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
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetPbftCore)
	defer net.close()

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)

	req := &Request{
		Timestamp: &gp.Timestamp{Seconds: 1, Nanos: 0},
		Payload:   txPacked,
		ReplicaId: uint64(generateBroadcaster(validatorCount)),
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
		inst.pbft.recvMsgSync(msgReq, uint64(msg.src))
	}

	err := net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if inst.id != 3 && blockHeight <= 1 {
			t.Errorf("Expected execution on replica %d", inst.id)
			continue
		}
		if inst.id == 3 && blockHeight > 1 {
			t.Errorf("Expected no execution")
			continue
		}
	}
}

func TestInconsistentPrePrepare(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetPbftCore)
	defer net.close()

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
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if blockHeight > 1 {
			t.Errorf("Expected no execution")
			continue
		}
	}
}

func TestViewChange(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, func(inst *instance) {
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
		msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: uint64(generateBroadcaster(validatorCount))}}}
		err = net.replicas[0].pbft.recvMsgSync(msg, msg.GetRequest().ReplicaId)
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
	if !ok || cp.SequenceNumber != 2 {
		t.Fatalf("Wrong new initial checkpoint: %+v",
			net.replicas[1].pbft.viewChangeStore)
	}

	msgList := net.replicas[1].pbft.assignSequenceNumbers(net.replicas[1].pbft.getViewChanges(), cp.SequenceNumber)
	if msgList[4] != "" || msgList[5] != "" || msgList[3] == "" {
		t.Fatalf("Wrong message list: %+v", msgList)
	}
}

func TestInconsistentDataViewChange(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetPbftCore)
	defer net.close()

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
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if blockHeight > 1 {
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

	for _, inst := range net.replicas {
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if blockHeight <= 1 {
			t.Errorf("Expected execution")
			continue
		}
	}
}

func TestViewChangeWithStateTransfer(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, makeTestnetPbftCore)
	defer net.close()

	var err error

	for _, inst := range net.replicas {
		inst.pbft.K = 2
		inst.pbft.L = 4
	}

	stsrc := net.replicas[3].pbft.sts.CompletionChannel()

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
		_ = net.replicas[0].pbft.recvRequest(makePP(i).Request)

		// clear all messages sent by primary
		net.msgs = net.msgs[:0]

		_ = net.replicas[0].pbft.recvPrePrepare(makePP(i))
		_ = net.replicas[1].pbft.recvPrePrepare(makePP(i))
		_ = net.replicas[2].pbft.recvPrePrepare(makePP(i))

		err = net.process()
		if err != nil {
			t.Fatalf("Processing failed: %s", err)
		}
	}

	fmt.Println("Done with stage 1")

	// Have primary now deliberately skip a sequence number to deadlock until viewchange
	_ = net.replicas[0].pbft.recvRequest(makePP(4).Request)

	// clear all messages sent by primary
	net.msgs = net.msgs[:0]

	// replace with fake messages
	_ = net.replicas[1].pbft.recvPrePrepare(makePP(5))
	_ = net.replicas[2].pbft.recvPrePrepare(makePP(5))
	_ = net.replicas[3].pbft.recvPrePrepare(makePP(4))

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	fmt.Println("Done with stage 2")

	for _, inst := range net.replicas {
		inst.pbft.sendViewChange()
	}

	fmt.Println("Done with stage 3")
	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	select {
	case <-stsrc:
		// State transfer for replica 3 is complete
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for state transfer to complete")
	}
	fmt.Println("Done with stage 4")

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}
	fmt.Println("Done with stage 5")

	for _, inst := range net.replicas {
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if blockHeight <= 4 {
			t.Errorf("Expected execution for inst %d, got blockheight of %d", inst.pbft.id, blockHeight)
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
	net := makeTestnet(validatorCount, func(inst *instance) {
		makeTestnetPbftCore(inst)
		inst.pbft.newViewTimeout = millisUntilTimeout * time.Millisecond
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

	broadcaster := uint64(generateBroadcaster(validatorCount))

	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, _ := proto.Marshal(tx)
	msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: broadcaster}}}
	msgPacked, _ := proto.Marshal(msg)

	go net.processContinually()

	ticker := time.NewTicker(millisUntilTimeout * time.Millisecond)

	// This will eventually trigger 1's request timeout
	// We check that one single timed out replica will not keep trying to change views by itself
	net.replicas[1].pbft.receive(msgPacked, broadcaster)
	fmt.Println("Debug: Sleeping 1")
	for i := 0; i < 5; i++ {
		<-ticker.C // Let the timeout interval pass twice to ensure it fires for replica 1
		fmt.Println("Debug: (still) Sleeping 1")
	}
	fmt.Println("Debug: Waking 1")

	// This will eventually trigger 3's request timeout, which will lead to a view change to 1.
	// However, we disable 1, which will disable the new-view going through.
	// This checks that replicas will automatically move to view 2 when the view change times out.
	// However, 2 does not know about the missing request, and therefore the request will not be
	// pre-prepared and finally executed.  This will lead to another view-change timeout, even on
	// the replicas that never saw the request (e.g. 0)
	// Finally, 3 will be new primary and pre-prepare the missing request.
	replica1Disabled = true
	net.replicas[3].pbft.receive(msgPacked, broadcaster)
	fmt.Println("Debug: Sleeping 2")
	for i := 0; i < 10; i++ {
		fmt.Println("Debug: (still) Sleeping 2")
		<-ticker.C // Let the timeout interval pass 10 times, but potentially much longer in a VM
	}
	fmt.Println("Debug: Waking 2")

	ticker.Stop()

	net.close()
	for i, inst := range net.replicas {
		if inst.pbft.view < 3 {
			t.Errorf("Should have reached view 3, got %d instead for replica %d", inst.pbft.view, i)
		}
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		blockHeightExpected := uint64(2)
		if blockHeight != blockHeightExpected {
			t.Errorf("Should have executed %d, got %d instead for replica %d", blockHeightExpected, blockHeight, i)
		}
	}
}

func TestFallBehind(t *testing.T) {
	validatorCount := 4
	net := makeTestnet(validatorCount, func(inst *instance) {
		makeTestnetPbftCore(inst)
		inst.pbft.K = 2
		inst.pbft.L = 2 * inst.pbft.K
	})
	defer net.close()

	execReq := func(iter int64, skipThree bool) {
		// Create a message of type `OpenchainMessage_CHAIN_TRANSACTION`
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}

		msg := &Message{&Message_Request{&Request{Payload: txPacked, ReplicaId: uint64(generateBroadcaster(validatorCount))}}}

		err = net.replicas[0].pbft.recvMsgSync(msg, msg.GetRequest().ReplicaId)
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

	inst := net.replicas[3].pbft

	// Send enough requests to get to a checkpoint quorum certificate with sequence number L+K
	execReq(1, true)
	for request := int64(2); uint64(request) <= inst.L+inst.K; request++ {
		execReq(request, false)
	}

	if !inst.sts.InProgress() {
		t.Fatalf("Replica did not detect that it has fallen behind.")
	}

	if len(inst.chkpts) != 0 {
		t.Fatalf("Expected no checkpoints, found %d", len(inst.chkpts))
	}

	if inst.h != inst.L+inst.K {
		t.Fatalf("Expected low water mark to be %d, got %d", inst.L+inst.K, inst.h)
	}

	// Send enough requests to get to a weak checkpoint certificate certain with sequence number L+K*2
	for request := int64(inst.L + inst.K + 1); uint64(request) <= inst.L+inst.K*2; request++ {
		execReq(request, false)
	}

	success := false

	for i := 0; i < 200; i++ { // Loops for up to 2 seconds waiting
		time.Sleep(10 * time.Millisecond)
		if !inst.sts.InProgress() {
			success = true
			break
		}
	}

	if !success {
		t.Fatalf("Request failed to complete state transfer within 2 seconds")
	}

	execReq(int64(inst.L+inst.K*2+1), false)

	if inst.lastExec != inst.L+inst.K*2+1 {
		t.Fatalf("Replica did not begin participating normally after state transfer completed")
	}
}

func executeStateTransferFromPBFT(pbft *pbftCore, ml *MockLedger, blockNumber, sequenceNumber uint64, mrls *map[pb.PeerID]*MockRemoteLedger) error {

	var chkpt *Checkpoint

	for i := uint64(1); i <= 3; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber - i,
			ReplicaId:      i,
			BlockNumber:    blockNumber - i,
			BlockHash:      base64.StdEncoding.EncodeToString(SimpleGetBlockHash(blockNumber - i)),
		}
		handle, _ := getValidatorHandle(uint64(i))
		(*mrls)[*handle].blockHeight = blockNumber - 2
		pbft.witnessCheckpoint(chkpt)
	}

	if !pbft.sts.InProgress() {
		return fmt.Errorf("Replica did not detect itself falling behind to initiate the state transfer")
	}

	result := pbft.sts.CompletionChannel()

	for i := 1; i < pbft.replicaCount; i++ {
		chkpt = &Checkpoint{
			SequenceNumber: sequenceNumber,
			ReplicaId:      uint64(i),
			BlockNumber:    blockNumber,
			BlockHash:      base64.StdEncoding.EncodeToString(SimpleGetBlockHash(blockNumber)),
		}
		handle, _ := getValidatorHandle(uint64(i))
		(*mrls)[*handle].blockHeight = blockNumber + 1
		pbft.checkpointStore[*chkpt] = true
	}

	pbft.witnessCheckpointWeakCert(chkpt)

	select {
	case <-time.After(time.Second * 2):
		return fmt.Errorf("Timed out waiting for state to catch up, error in state transfer")
	case <-result:
		// Do nothing, continue the test
	}

	if size, _ := pbft.ledger.GetBlockchainSize(); size != blockNumber+1 { // Will never fail
		return fmt.Errorf("Blockchain should be caught up to block %d, but is only %d tall", blockNumber, size)
	}

	block, err := pbft.ledger.GetBlock(blockNumber)

	if nil != err {
		return fmt.Errorf("Error retrieving last block in the mock chain.")
	}

	if stateHash, _ := pbft.ledger.GetCurrentStateHash(); !bytes.Equal(stateHash, block.StateHash) {
		return fmt.Errorf("Current state does not validate against the latest block.")
	}

	return nil
}

func TestCatchupFromPBFTSimple(t *testing.T) {
	rols := make(map[pb.PeerID]consensus.ReadOnlyLedger)
	mrls := make(map[pb.PeerID]*MockRemoteLedger)
	for i := uint64(0); i <= 3; i++ {
		peerID, _ := getValidatorHandle(i)
		l := &MockRemoteLedger{}
		rols[*peerID] = l
		mrls[*peerID] = l
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(&rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	config := loadConfig()
	pbft := newPbftCore(0, config, nil, ml)
	pbft.K = 2
	pbft.L = 4
	pbft.replicaCount = 4

	if err := executeStateTransferFromPBFT(pbft, ml, 7, 10, &mrls); nil != err {
		t.Fatalf("TestCatchupFromPBFT simple case: %s", err)
	}
}

func TestCatchupFromPBFTDivergentSeqBlock(t *testing.T) {
	rols := make(map[pb.PeerID]consensus.ReadOnlyLedger)
	mrls := make(map[pb.PeerID]*MockRemoteLedger)
	for i := uint64(0); i <= 3; i++ {
		peerID, _ := getValidatorHandle(i)
		l := &MockRemoteLedger{}
		rols[*peerID] = l
		mrls[*peerID] = l
	}

	// Test from blockheight of 1, with valid genesis block
	ml := NewMockLedger(&rols, nil)
	ml.PutBlock(0, SimpleGetBlock(0))
	config := loadConfig()
	pbft := newPbftCore(0, config, nil, ml)
	pbft.K = 2
	pbft.L = 4
	pbft.replicaCount = 4

	// Test to make sure that the block number and sequence number can diverge
	if err := executeStateTransferFromPBFT(pbft, ml, 7, 100, &mrls); nil != err {
		t.Fatalf("TestCatchupFromPBFTDivergentSeqBlock separated block/seqnumber: %s", err)
	}
}

func TestPbftF0(t *testing.T) {
	net := makeTestnet(1, func(inst *instance) {
		os.Setenv("OPENCHAIN_OBCPBFT_GENERAL_N", fmt.Sprintf("%d", inst.net.N)) // TODO, a little hacky, but needed for state transfer not to get upset
		os.Setenv("OPENCHAIN_OBCPBFT_GENERAL_F", "0")                           // TODO, a little hacky, but needed for state transfer not to get upset
		defer func() {
			os.Unsetenv("OPENCHAIN_OBCPBFT_GENERAL_N")
			os.Unsetenv("OPENCHAIN_OBCPBFT_GENERAL_F")
		}()
		config := loadConfig()
		inst.pbft = newPbftCore(uint64(inst.id), config, inst, inst)
		inst.pbft.replicaCount = inst.net.N
		inst.pbft.f = inst.net.f
		inst.deliver = func(msg []byte, senderHandle *pb.PeerID) {
			senderID, _ := getValidatorID(senderHandle)
			inst.pbft.receive(msg, senderID)
		}

	})
	defer net.close()

	// Create a message of type: `OpenchainMessage_CHAIN_TRANSACTION`
	txTime := &gp.Timestamp{Seconds: 1, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime, Payload: []byte("TestNetwork")}
	txPacked, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	err = net.replicas[0].pbft.request(txPacked, 0)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		blockHeight, _ := inst.ledger.GetBlockchainSize()
		if blockHeight <= 1 {
			t.Errorf("Instance %d did not execute transaction", inst.id)
			continue
		}
		if blockHeight != 2 {
			t.Errorf("Instance %d executed more than one transaction", inst.id)
			continue
		}
		highestBlock, _ := inst.ledger.GetBlock(blockHeight - 1)
		if !reflect.DeepEqual(highestBlock.Transactions[0].Payload, txPacked) {
			t.Errorf("Instance %d executed wrong transaction, %x should be %x",
				inst.id, highestBlock.Transactions[0].Payload, txPacked)
		}
	}
}

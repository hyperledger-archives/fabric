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

package pbft

import (
	"fmt"
	gp "google/protobuf"
	"os"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	pb "github.com/openblockchain/obc-peer/protos"
)

func TestEnvOverride(t *testing.T) {

	mock := NewMock()
	instance := New(mock)

	key := "general.name"                    // for a key that exists
	envName := "OPENCHAIN_PBFT_GENERAL_NAME" // env override name
	overrideValue := "overide_test"          // value to override default value with

	// test key
	if ok := instance.config.IsSet("general.name"); !ok {
		t.Fatalf("Cannot test env override because \"%s\" does not seem to be set", key)
	}

	os.Setenv(envName, overrideValue)
	// The override config value will cause other calls to fail unless unset.
	defer func() {
		os.Unsetenv(envName)
	}()

	if ok := instance.config.IsSet("general.name"); !ok {
		t.Fatalf("Env override in place, and key \"%s\" is not set", key)
	}

	// read key
	configVal := instance.config.GetString("general.name")
	if configVal != overrideValue {
		t.Fatalf("Env override in place, expected key \"%s\" to be \"%s\" but instead got \"%s\"", key, overrideValue, configVal)
	}

}

func TestGetParam(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	key := "general.name" // for a key that exists
	realVal := "pbft"     // expected value

	testVal, err := instance.getParam(key) // read key
	// Error should be nil, since the key exists.
	if err != nil {
		t.Fatalf("Error when retrieving value for existing key %s: %s", key, err)
	}
	// Values should match.
	if testVal != realVal {
		t.Fatalf("Expected value %s for key %s, got %s instead", realVal, key, testVal)
	}

	// read key
	key = "non.existing.key"
	_, err = instance.getParam(key)
	// Error should not be nil, since the key does not exist.
	if err == nil {
		t.Fatal("Expected error since retrieving value for non-existing key, got nil instead")
	}
}

func TestRecvRequest(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.replicaCount = 5

	txTime := &gp.Timestamp{Seconds: 2000, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal TX : %s", err)
	}
	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CHAIN_TRANSACTION,
		Payload: txPacked,
	}
	err = instance.RecvMsg(msgWrapped)
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}

	msgOut := mock.broadcasted[0]
	if msgOut == nil || msgOut.Type != pb.OpenchainMessage_CONSENSUS {
		t.Fatalf("expected broadcast after request, received %s", msgWrapped.Type)
	}

	msgRaw := msgOut.Payload
	msgReq := &Message{}
	err = proto.Unmarshal(msgRaw, msgReq)
	if err != nil {
		t.Fatal("could not unmarshal message")
	}
	// TODO test for correct transaction passing
}

func TestRecvMsg(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.id = 0
	instance.replicaCount = 5

	nestedMsg := &Message{&Message_Request{&Request{
		Payload: []byte("hello world"),
	}}}
	newPayload, err := proto.Marshal(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
	}
	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: newPayload,
	}
	err = instance.RecvMsg(msgWrapped)
	if err != nil {
		t.Fatalf("Failed to handle pbft message: %s", err)
	}

	if len(mock.broadcasted) != 1 {
		t.Fatalf("expected message, got %d", len(mock.broadcasted))
	}

	msgOut := mock.broadcasted[0]
	if msgOut.Type != pb.OpenchainMessage_CONSENSUS {
		t.Fatalf("expected CONSENSUS, received %s", msgOut.Type)
	}
	msg := &Message{}
	err = proto.Unmarshal(msgOut.Payload, msg)
	if err != nil {
		t.Fatal("could not unmarshal message")
	}
	if msg := msg.GetPrePrepare(); msg == nil {
		t.Fatal("expected pre-prepare after request")
	}
}

func TestMaliciousPrePrepare(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.id = 1
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
	newPayload, err := proto.Marshal(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
	}
	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: newPayload,
	}
	err = instance.RecvMsg(msgWrapped)
	if err != nil {
		t.Fatalf("Failed to handle pbft message: %s", err)
	}

	if len(mock.broadcasted) != 0 {
		t.Fatalf("expected to ignore malicious pre-prepare")
	}
}

func TestIncompletePayload(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.id = 1
	instance.replicaCount = 5

	checkMsg := func(msg *Message, errMsg string, args ...interface{}) {
		newPayload, err := proto.Marshal(msg)
		if err != nil {
			t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
		}
		msgWrapped := &pb.OpenchainMessage{
			Type:    pb.OpenchainMessage_CONSENSUS,
			Payload: newPayload,
		}
		_ = instance.RecvMsg(msgWrapped)
		if len(mock.broadcasted) != 0 {
			t.Errorf(errMsg, args...)
		}
	}

	checkMsg(&Message{}, "should reject empty message")
	checkMsg(&Message{&Message_Request{&Request{}}}, "should reject empty request")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{}}}, "should reject empty pre-prepare")
	checkMsg(&Message{&Message_PrePrepare{&PrePrepare{SequenceNumber: 1}}}, "should reject empty pre-prepare")
}

// =============================================================================
// Fake network structures
// =============================================================================

type mockCPI struct {
	broadcasted []*pb.OpenchainMessage
	executed    [][]*pb.Transaction
}

type taggedMsg struct {
	id  int
	msg *pb.OpenchainMessage
}

type testnet struct {
	replicas []*instance
	msgs     []taggedMsg
}

type instance struct {
	id       int
	plugin   *Plugin
	net      *testnet
	executed [][]*pb.Transaction
}

// =============================================================================
// Interface implementations
// =============================================================================

func (mock *mockCPI) Broadcast(msg *pb.OpenchainMessage) error {
	mock.broadcasted = append(mock.broadcasted, msg)
	return nil
}

func (mock *mockCPI) Unicast(msgPayload []byte, dest string) error {
	panic("not implemented yet")
}

func (mock *mockCPI) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	mock.executed = append(mock.executed, txs)
	return []byte("hash"), make([]error, len(txs)+1)
}

func (inst *instance) Broadcast(msg *pb.OpenchainMessage) error {
	net := inst.net
	net.msgs = append(net.msgs, taggedMsg{inst.id, msg})
	return nil
}

func (*instance) Unicast(msgPayload []byte, receiver string) error {
	panic("not implemented yet")
}

func (inst *instance) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	inst.executed = append(inst.executed, txs)
	return []byte("hash"), nil
}

func NewMock() *mockCPI {
	mock := &mockCPI{
		make([]*pb.OpenchainMessage, 0),
		make([][]*pb.Transaction, 0),
	}
	return mock
}

func (net *testnet) process() error {
	for len(net.msgs) > 0 {
		msgs := net.msgs
		net.msgs = nil

		for _, taggedMsg := range msgs {
			for i, replica := range net.replicas {
				if i == taggedMsg.id {
					continue
				}
				err := replica.plugin.RecvMsg(taggedMsg.msg)
				if err != nil {
					return nil
				}
			}
		}
	}

	return nil
}

func TestNetwork(t *testing.T) {

	fmt.Print("\n")

	const f = 2
	const replicaCount = 3*f + 1
	net := &testnet{}
	for i := 0; i < replicaCount; i++ {
		inst := &instance{id: i, net: net}
		inst.plugin = New(inst)
		inst.plugin.id = uint64(i)
		inst.plugin.replicaCount = replicaCount
		inst.plugin.f = f
		net.replicas = append(net.replicas, inst)
	}

	// Create a message of type: `OpenchainMessage_CHAIN_TRANSACTION`
	txTime := &gp.Timestamp{Seconds: 2001, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txPacked, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CHAIN_TRANSACTION,
		Payload: txPacked,
	}
	err = net.replicas[0].plugin.RecvMsg(msg)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("Processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if len(inst.executed) == 0 {
			t.Errorf("Instance %d did not execute transaction", inst.id)
			continue
		}
		if len(inst.executed[0]) != 1 {
			t.Errorf("Instance %d executed more than one transaction", inst.id)
			continue
		}
		if !reflect.DeepEqual(inst.executed[0][0], tx) {
			t.Errorf("Instance %d executed wrong transaction, %s should be %s",
				inst.id, inst.executed[0], tx)
		}
	}
}

func TestCheckpoint(t *testing.T) {
	const f = 1
	const replicaCount = 3*f + 1
	net := &testnet{}
	for i := 0; i < replicaCount; i++ {
		inst := &instance{id: i, net: net}
		inst.plugin = New(inst)
		inst.plugin.id = uint64(i)
		inst.plugin.replicaCount = replicaCount
		inst.plugin.f = f
		inst.plugin.K = 2
		net.replicas = append(net.replicas, inst)
	}

	execReq := func(iter int64) {
		// Create a message of type: `OpenchainMessage_CHAIN_TRANSACTION`
		txTime := &gp.Timestamp{Seconds: iter, Nanos: 0}
		tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
		txPacked, err := proto.Marshal(tx)
		if err != nil {
			t.Fatalf("Failed to marshal TX block: %s", err)
		}
		msg := &pb.OpenchainMessage{
			Type:    pb.OpenchainMessage_CHAIN_TRANSACTION,
			Payload: txPacked,
		}
		err = net.replicas[0].plugin.RecvMsg(msg)
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
		if len(inst.plugin.chkpts) != 1 {
			t.Errorf("expected 1 checkpoint, found %d", len(inst.plugin.chkpts))
			continue
		}

		if _, ok := inst.plugin.chkpts[2]; !ok {
			t.Errorf("expected checkpoint for seqNo 2, got %s", inst.plugin.chkpts)
			continue
		}

		if inst.plugin.h != 2 {
			t.Errorf("expected low water mark to be 2, is %d", inst.plugin.h)
			continue
		}
	}
}

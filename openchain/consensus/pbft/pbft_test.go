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
	gp "google/protobuf"
	"os"
	"testing"

	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
)

// =============================================================================
// Mock set-up
// =============================================================================

type mockCPI struct {
	broadcasted []*pb.OpenchainMessage
	executed    [][]*pb.Transaction
}

func NewMock() *mockCPI {
	mock := &mockCPI{
		make([]*pb.OpenchainMessage, 0),
		make([][]*pb.Transaction, 0),
	}
	return mock
}

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

// =============================================================================
// Tests begin here
// =============================================================================

func TestEnvOverride(t *testing.T) {
	// For a key that exists
	key := "general.name"
	// Env override name
	envName := "OPENCHAIN_PBFT_GENERAL_NAME"
	// Value to override default value with
	overrideValue := "overide_test"

	mock := NewMock()
	instance := New(mock)

	// Test key.
	if ok := instance.config.IsSet("general.name"); !ok {
		t.Fatalf("Cannot test env override because \"%s\" does not seem to be set", key)
	}

	os.Setenv(envName, overrideValue)
	// The override config value will cause other calls to fail unless unset
	defer func() {
		os.Unsetenv(envName)
	}()

	if ok := instance.config.IsSet("general.name"); !ok {
		t.Fatalf("Env override in place, but key \"%s\" is not set", key)
	}

	// Read key
	configVal := instance.config.GetString("general.name")
	if configVal != overrideValue {
		t.Fatalf("Env override in place, expected key \"%s\" to be \"%s\" but got \"%s\" instead", key, overrideValue, configVal)
	}

}

func TestGetParam(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	// For a key that exists
	key := "general.name"
	// Expected value
	realVal := "pbft"
	// Read key
	testVal, err := instance.getParam(key)
	// Error should be nil, since the key exists
	if err != nil {
		t.Fatalf("Error when retrieving value for existing key %s: %s", key, err)
	}
	// Values should match
	if testVal != realVal {
		t.Fatalf("Expected value %s for key %s, got %s instead", realVal, key, testVal)
	}

	// Read key
	key = "non.existing.key"
	_, err = instance.getParam(key)
	// Error should not be nil, since the key does not exist
	if err == nil {
		t.Fatal("Expected error since retrieving value for non-existing key, got nil instead")
	}
}

func TestLeader(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	var ans bool
	ans = instance.setLeader(true)
	if !ans {
		t.Fatalf("Unable to set validating peer as leader")
	}
	ans = instance.isLeader()
	if !ans {
		t.Fatalf("Unable to query validating peer for leader status")
	}
}

func TestRecvMsg(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.id = 0
	instance.replicaCount = 5

	reqMsg := &Message{&Message_Request{&Request{
		Payload: []byte("hello world"),
	}}}
	newPayload, _ := proto.Marshal(reqMsg)
	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: newPayload,
	}
	err := instance.RecvMsg(msgWrapped)
	if err != nil {
		t.Fatalf("Failed to handle PBFT message: %s", err)
	}

	if len(mock.broadcasted) != 1 {
		t.Fatalf("Expected a message, got %d messages instead", len(mock.broadcasted))
	}

	msgOut := mock.broadcasted[0]
	if msgOut.Type != pb.OpenchainMessage_CONSENSUS {
		t.Fatalf("Expected CONSENSUS, received: %s", msgOut.Type)
	}

	msg := &Message{}
	err = proto.Unmarshal(msgOut.Payload, msg)
	if err != nil {
		t.Fatal("Cannot unmarshal message")
	}

	if preprep := msg.GetPrePrepare(); preprep == nil {
		t.Fatal("Expected pre-prepare after request")
	}
}

func TestRecvRequest(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.replicaCount = 5

	txTime := &gp.Timestamp{Seconds: 2000, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txBlockPacked, _ := proto.Marshal(txBlock)
	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_REQUEST,
		Payload: txBlockPacked,
	}
	err := instance.RecvMsg(msgWrapped)
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}

	msgOut := mock.broadcasted[0]
	if msgOut == nil || msgOut.Type != pb.OpenchainMessage_CONSENSUS {
		t.Fatalf("Expected broadcast after request, received: %s", msgWrapped.Type)
	}

	msgRaw := msgOut.Payload
	msgReq := &Message{}
	err = proto.Unmarshal(msgRaw, msgReq)
	if err != nil {
		t.Fatal("Cannot unmarshal message")
	}

	// TODO test for correct transaction passing
}

// =============================================================================
// Test with fake network
// =============================================================================

type testnetwork struct {
	replicas []*instance
}

type instance struct {
	id     int
	plugin *Plugin
	net    *testnetwork
}

func (inst *instance) Broadcast(msg *pb.OpenchainMessage) error {
	for i, replica := range inst.net.replicas {
		if i == inst.id {
			continue
		}
		replica.plugin.RecvMsg(msg)
	}

	return nil
}

func (*instance) Unicast(msgPayload []byte, receiver string) error {
	panic("Not implemented yet")
}

func (*instance) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	panic("Not implemented yet")
}

func TestNetwork(t *testing.T) {
	const f = 2
	const replicaCount = 2*f + 1

	net := &testnetwork{make([]*instance, replicaCount)}
	for i := range net.replicas {
		inst := &instance{id: i, net: net}
		inst.plugin = New(inst)
		inst.plugin.id = uint64(i)
		inst.plugin.replicaCount = replicaCount
		net.replicas[i] = inst
	}

	net.replicas[0].plugin.setLeader(true)

	// Create a message of type: `OpenchainMessage_REQUEST`
	txTime := &gp.Timestamp{Seconds: 2001, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txBlockPacked, err := proto.Marshal(txBlock)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	err = net.replicas[0].plugin.Request(txBlockPacked)
	if err != nil {
		t.Fatalf("Request failed: %s", err)
	}
}

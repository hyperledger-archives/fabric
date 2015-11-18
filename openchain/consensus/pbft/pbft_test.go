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

	key := "general.name"                    // for a key that exists
	envName := "OPENCHAIN_PBFT_GENERAL_NAME" // env override name
	overrideValue := "overide_test"          // value to override default value with

	mock := NewMock()
	instance := New(mock)

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

func TestRecvRequest(t *testing.T) {
	mock := NewMock()
	instance := New(mock)
	instance.replicaCount = 5

	txTime := &gp.Timestamp{Seconds: 2000, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txBlockPacked, err := proto.Marshal(txBlock)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	msgWrapped := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_REQUEST,
		Payload: txBlockPacked,
	}
	err = instance.RecvMsg(msgWrapped)
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}

	msgOut := mock.broadcastMsg[0]
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

	if len(mock.broadcastMsg) != 1 {
		t.Fatalf("expected message, got %d", len(mock.broadcastMsg))
	}

	msgOut := mock.broadcastMsg[0]
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

// =============================================================================
// Fake network structures
// =============================================================================

type mockCPI struct {
	broadcastMsg []*pb.OpenchainMessage
	execTx       [][]*pb.Transaction
}

type taggedMsg struct {
	id  int
	msg *pb.OpenchainMessage
}

type testnetwork struct {
	replicas []*instance
	msgs     []taggedMsg
}

type instance struct {
	id       int
	plugin   *Plugin
	net      *testnetwork
	executed [][]*pb.Transaction
}

// =============================================================================
// Interface implementations
// =============================================================================

func (mock *mockCPI) Broadcast(msg *pb.OpenchainMessage) error {
	mock.broadcastMsg = append(mock.broadcastMsg, msg)
	return nil
}

func (mock *mockCPI) Unicast(msgPayload []byte, dest string) error {
	panic("not implemented yet")
}

func (mock *mockCPI) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	mock.execTx = append(mock.execTx, txs)
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

func (net *testnetwork) process() error {
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

	fmt.Print("\n\n\n")

	const f = 2
	const nreplica = 3*f + 1
	net := &testnetwork{}
	for i := 0; i < nreplica; i++ {
		inst := &instance{id: i, net: net}
		inst.plugin = New(inst)
		inst.plugin.id = uint64(i)
		inst.plugin.replicaCount = nreplica
		inst.plugin.f = f
		net.replicas = append(net.replicas, inst)
	}

	net.replicas[0].plugin.setLeader(true)

	// Create a message of type: `OpenchainMessage_REQUEST`
	txTime := &gp.Timestamp{Seconds: 2001, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txs := []*pb.Transaction{tx}
	txBlock := &pb.TransactionBlock{Transactions: txs}
	txBlockPacked, err := proto.Marshal(txBlock)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_REQUEST,
		Payload: txBlockPacked,
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
		} else if !reflect.DeepEqual(inst.executed[0], txs) {
			t.Errorf("Instance %d executed wrong transaction, %s should be %s",
				inst.id, inst.executed[0], txs)
		}
	}
}

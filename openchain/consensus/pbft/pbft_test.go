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
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	pb "github.com/openblockchain/obc-peer/protos"
)

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
		t.Fatalf("Env override in place, and key \"%s\" is not set", key)
	}

	// Read key
	configVal := instance.config.GetString("general.name")
	if configVal != overrideValue {
		t.Fatalf("Env override in place, expected key \"%s\" to be \"%s\" but instead got \"%s\"", key, overrideValue, configVal)
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

type mockCpi struct {
	broadcastMsg []*pb.OpenchainMessage
	execTx       [][]*pb.Transaction
}

func NewMock() *mockCpi {
	mock := &mockCpi{
		make([]*pb.OpenchainMessage, 0),
		make([][]*pb.Transaction, 0),
	}
	return mock
}

func (mock *mockCpi) Broadcast(msg *pb.OpenchainMessage) error {
	mock.broadcastMsg = append(mock.broadcastMsg, msg)
	return nil
}

func (mock *mockCpi) Unicast(msg []byte, dest string) error {
	panic("not implemented")
}

func (mock *mockCpi) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	mock.execTx = append(mock.execTx, txs)
	return []byte("hash"), make([]error, len(txs)+1)
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

//
// Test with fake network
//

type taggedmsg struct {
	id  int
	msg *pb.OpenchainMessage
}

type testnetwork struct {
	replicas []*instance
	msgs     []taggedmsg
}

type instance struct {
	id       int
	plugin   *Plugin
	net      *testnetwork
	executed [][]*pb.Transaction
}

func (*instance) Unicast(payload []byte, receiver string) error { panic("invalid") }
func (inst *instance) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	inst.executed = append(inst.executed, txs)
	return []byte("hash"), nil
}

func (inst *instance) Broadcast(msg *pb.OpenchainMessage) error {
	net := inst.net
	net.msgs = append(net.msgs, taggedmsg{inst.id, msg})
	return nil
}

func (net *testnetwork) process() error {
	for len(net.msgs) > 0 {
		msgs := net.msgs
		net.msgs = nil

		for _, taggedmsg := range msgs {
			for i, replica := range net.replicas {
				if i == taggedmsg.id {
					continue
				}
				err := replica.plugin.RecvMsg(taggedmsg.msg)
				if err != nil {
					return nil
				}
			}
		}
	}

	return nil
}

func TestNetwork(t *testing.T) {
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
		t.Fatalf("request failed: %s", err)
	}

	err = net.process()
	if err != nil {
		t.Fatalf("processing failed: %s", err)
	}

	for _, inst := range net.replicas {
		if len(inst.executed) == 0 {
			t.Errorf("instance %d did not execute transaction", inst.id)
		} else if !reflect.DeepEqual(inst.executed[0], txs) {
			t.Errorf("instance %d executed wrong transaction, %s should be %s",
				inst.id, inst.executed[0], txs)
		}
	}
}

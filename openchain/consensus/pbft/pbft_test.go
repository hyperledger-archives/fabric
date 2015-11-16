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
	"bytes"
	gp "google/protobuf"
	"os"
	"testing"

	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
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
	broadcastMsg [][]byte
	execTx       [][]*pb.Transaction
}

func NewMock() *mockCpi {
	mock := &mockCpi{
		make([][]byte, 0),
		make([][]*pb.Transaction, 0),
	}
	return mock
}

func (mock *mockCpi) Broadcast(msg []byte) error {
	mock.broadcastMsg = append(mock.broadcastMsg, msg)
	return nil
}

func (mock *mockCpi) Unicast(msg []byte, dest string) error {
	panic("not implemented")
}

func (mock *mockCpi) ExecTXs(ctx context.Context, txs []*pb.Transaction) ([]byte, []error) {
	mock.execTx = append(mock.execTx, txs)
	return []byte("hash"), make([]error, len(txs)+1)
}

func TestRecvRequest(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	txTime := &gp.Timestamp{Seconds: 2000, Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txBlockPacked, err := proto.Marshal(txBlock)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	err = instance.Request(txBlockPacked)
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}

	msgRaw := mock.broadcastMsg[0]
	if msgRaw == nil {
		t.Fatalf("expected broadcast after request")
	}
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

	nestedMsg := &Message{&Message_Request{&Request{
		Payload: []byte("hello world"),
	}}}
	newPayload, err := proto.Marshal(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
	}
	err = instance.RecvMsg(newPayload)
	if err != nil {
		t.Fatalf("Failed to handle pbft message: %s", err)
	}

	if len(mock.broadcastMsg) != 0 {
		t.Fatal("unexpected message")
	}

	// msgRaw := mock.broadcastMsg[0]
	// msg := &Message{}
	// err = proto.Unmarshal(msgRaw, msg)
	// if err != nil {
	// 	t.Fatal("could not unmarshal message")
	// }
}

func TestStoreRetrieve(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	// Create a `REQUEST` message
	req := &Request{Payload: []byte("hello world")}
	// Marshal it
	reqPacked, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("Error marshalling request")
	}

	// Get its hash
	digest := hashMsg(reqPacked)

	// Store it
	count := instance.storeRequest(digest, req)

	if count != 1 {
		t.Fatalf("Expected message count in map is 1, instead got: %d", count)
	}

	t.Logf("Map: %+v\n", instance.msgStore)

	newReq, err := instance.retrieveRequest(digest)
	if err != nil {
		t.Fatalf("Error retrieving REQUEST message from map")
	}
	// Marshal it
	newReqPacked, err := proto.Marshal(newReq)
	if err != nil {
		t.Fatalf("Error marshalling retrieved REQUEST message")
	}

	// Are the two messages the same?
	if !bytes.Equal(reqPacked, newReqPacked) {
		t.Logf("Retrieved REQUEST message is not identical to original one")
	}
}

//
// Test with fake network
//

type testnetwork struct {
	replicas []*instance
}

type instance struct {
	id     int
	plugin *Plugin
	net    *testnetwork
}

func (*instance) Unicast(payload []byte, receiver string) error { panic("invalid") }
func (*instance) ExecTXs(ctx context.Context, txs []*pb.Transaction) ([]byte, []error) {
	panic("invalid")
}

func (inst *instance) Broadcast(payload []byte) error {
	for i, replica := range inst.net.replicas {
		if i == inst.id {
			continue
		}

		replica.plugin.RecvMsg(payload)
	}

	return nil
}

func TestNetwork(t *testing.T) {
	const f = 2
	const nreplica = 2*f + 1
	net := &testnetwork{make([]*instance, nreplica)}
	for i := range net.replicas {
		inst := &instance{id: i, net: net}
		inst.plugin = New(inst)
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
		t.Fatalf("request failed: %s", err)
	}
}

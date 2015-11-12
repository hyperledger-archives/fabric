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
	"testing"

	"github.com/openblockchain/obc-peer/openchain/consensus/helper"

	"github.com/golang/protobuf/proto"
)

func TestGetParam(t *testing.T) {

	// Create new algorithm instance.
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	// For a key that exists.
	key := "general.name"
	// Expected value.
	realVal := "pbft"
	// Read key.
	testVal, err := instance.getParam(key)
	// Error should be nil, since the key exists.
	if err != nil {
		t.Fatalf("Error when retrieving value for existing key %s: %s", key, err)
	}
	// Values should match.
	if testVal != realVal {
		t.Fatalf("Expected value %s for key %s, got %s instead.", realVal, key, testVal)
	}

	// Read key.
	key = "non.existing.key"
	_, err = instance.getParam(key)
	// Error should not be nil, since the key does not exist.
	if err == nil {
		t.Fatal("Expected error since retrieving value for non-existing key, got nil instead.")
	}
}

func TestLeader(t *testing.T) {

	// Create new algorithm instance.
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	// Do not access through `helperInstance.consenter.`
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
	execTx       [][]byte
}

func NewMock() *mockCpi {
	mock := &mockCpi{
		make([][]byte, 0),
		make([][]byte, 0),
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

func (mock *mockCpi) ExecTx(msg []byte) ([]byte, error) {
	mock.execTx = append(mock.execTx, msg)
	return []byte("hash"), nil
}

func TestRequest(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	var err error

	req := []byte("hi there")
	err = instance.Request(req)
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}

	msgRaw := mock.broadcastMsg[0]
	msg := &Message{}
	err = proto.Unmarshal(msgRaw, msg)
	if err != nil {
		t.Fatal("could not unmarshal message")
	}
	if !bytes.Equal(msg.GetRequest().Transaction, req) {
		t.Fatalf("expected request broadcast, got something else: %x != %x", req, msg)
	}
}

func TestRecvMsg(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	var err error

	nestedMsg := &Message{
		&Message_Request{&Request{Transaction: []byte("hi there")}},
	}
	newPayload, err := proto.Marshal(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
	}
	err = instance.RecvMsg(newPayload)
	if err != nil {
		t.Fatalf("Failed to handle message: %s", err)
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
func (*instance) ExecTx([]byte) ([]byte, error) {
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

	net.replicas[0].plugin.Request([]byte("hi there"))
}

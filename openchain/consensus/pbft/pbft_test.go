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
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus/helper"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
)

func TestEnvOverride(t *testing.T) {

	// For a key that exists
	key := "general.name"
	// Env override name
	envName := "OPENCHAIN_PBFT_GENERAL_NAME"
	// Value to override default value with
	overrideValue := "overide_test"

	// Create new algorithm instance
	helperInstance := helper.New()
	instance := New(helperInstance)

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

	// Create new algorithm instance
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

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

	// Create new algorithm instance
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	// Do not access through `helperInstance.consenter`
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

	// Create new algorithm instance
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	// Do not access through `helperInstance.consenter`
	var err error

	// Create a message of type: `OpenchainMessage_REQUEST`
	txTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_NEW, Timestamp: txTime}
	txBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txBlockPacked, err := proto.Marshal(txBlock)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_REQUEST,
		Payload: txBlockPacked,
	}
	err = instance.RecvMsg(msg)
	if err != nil {
		t.Fatalf("Failed to handle OpenchainMessage:%s message: %s", msg.Type, err)
	}

	// Create a message of type: `OpenchainMessage_CONSENSUS`
	msg.Type = pb.OpenchainMessage_CONSENSUS
	nestedMsg := &Unpack{
		Type:    Unpack_PREPARE,
		Payload: []byte("hello world"),
	}
	newPayload, err := proto.Marshal(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
	}
	msg.Payload = newPayload
	err = instance.RecvMsg(msg)
	if err != nil {
		t.Fatalf("Failed to handle message OpenchainMessage:%s message: %s", msg.Type, err)
	}
}

func TestStoreRetrieve(t *testing.T) {

	// Create new algorithm instance
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	// Create a `REQUEST` message
	reqMsg := &Request2{Payload: []byte("hello world")}
	// Marshal it
	reqMsgPacked, err := proto.Marshal(reqMsg)
	if err != nil {
		t.Fatalf("Error marshalling REQUEST message")
	}

	// Get its hash
	digest := hashMsg(reqMsgPacked)

	// Store it
	count := instance.storeRequest(digest, reqMsg)

	if count != 1 {
		t.Fatalf("Expected message count in map is 1, instead got: %d", count)
	}

	t.Logf("Map: %+v\n", instance.msgStore)

	newMsg, err := instance.retrieveRequest(digest)
	if err != nil {
		t.Fatalf("Error retrieving REQUEST message from map")
	}
	// Marshal it
	newMsgPacked, err := proto.Marshal(newMsg)
	if err != nil {
		t.Fatalf("Error marshalling retrieved REQUEST message")
	}

	// Are the two messages the same?
	if !bytes.Equal(reqMsgPacked, newMsgPacked) {
		t.Logf("Retrieved REQUEST message is not identical to original one")
	}
}

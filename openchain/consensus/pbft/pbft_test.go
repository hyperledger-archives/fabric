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

func TestRequest(t *testing.T) {
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	var err error

	err = instance.Request([]byte("hi there"))
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}
}

func TestRecvMsg(t *testing.T) {

	// Create new algorithm instance.
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)

	// Do not access through `helperInstance.consenter.`
	var err error

	nestedMsg := &Unpack{
		Type:    Unpack_PREPARE,
		Payload: []byte("hello world"),
	}
	newPayload, err := proto.Marshal(nestedMsg)
	if err != nil {
		t.Fatalf("Failed to marshal payload for CONSENSUS message: %s", err)
	}
	err = instance.RecvMsg(newPayload)
	if err != nil {
		t.Fatalf("Failed to handle message: %s", err)
	}
}

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

package controller

import (
	gp "google/protobuf"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus/pbft"
	pb "github.com/openblockchain/obc-peer/protos"
)

func TestHandleMsg(t *testing.T) {

	var err error

	helper := GetHelper()

	// Message of the wrong type.
	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_UNDEFINED,
		Payload: []byte("hello world"),
	}
	err = helper.HandleMsg(msg)
	if err == nil {
		t.Fatalf("Helper shouldn't handle OpenchainMessage:%s message: %s", msg.Type, err)
	}

	// Create a message of type: `OpenchainMessage_REQUEST`.
	txTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txBlockPacked, err := proto.Marshal(txBlock)
	if err != nil {
		t.Fatalf("Failed to marshal TX block: %s", err)
	}
	msg = &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_REQUEST,
		Payload: txBlockPacked,
	}
	err = helper.HandleMsg(msg)
	if err != nil {
		t.Fatalf("Failed to handle OpenchainMessage:%s message: %s", msg.Type, err)
	}

	// Create a message of type: `OpenchainMessage_CONSENSUS`.
	msg = &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: []byte("hello world"),
	}
	nestedMsg := &pbft.Unpack{
		Type:    pbft.Unpack_PREPARE,
		Payload: []byte("hello world"),
	}
	newPayload, _ := proto.Marshal(nestedMsg)
	msg.Payload = newPayload
	err = helper.HandleMsg(msg)
	if err != nil {
		t.Fatalf("Failed to handle OpenchainMessage:%s message: %s", msg.Type, err)
	}
}

func TestBroadcastMessage(t *testing.T) {

	helper := GetHelper()

	msgPayload := []byte("hello world")

	err := helper.Broadcast(msgPayload)
	if err != nil {
		t.Fatalf("Failed to broadcast message: %s", err)
	}
}

// TODO: Write unit test for ExecTXs().

func TestUnicastMessage(t *testing.T) {

	helper := GetHelper()

	msgPayload := []byte("hello world")
	receiver := "vp2" // TODO: Replace with proper receiver.

	err := helper.Unicast(msgPayload, receiver)
	if err != nil {
		t.Fatalf("Failed to unicast message to %s: %s", receiver, err)
	}
}

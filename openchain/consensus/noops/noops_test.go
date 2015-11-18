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

package noops

import (
	gp "google/protobuf"
	"testing"
	"time"

	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
)

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

func (mock *mockCPI) Unicast(msg []byte, dest string) error {
	panic("not implemented yet")
}

func (mock *mockCPI) ExecTXs(txs []*pb.Transaction) ([]byte, []error) {
	mock.executed = append(mock.executed, txs)
	return []byte("hash"), make([]error, len(txs)+1)
}

func TestRecvMsg(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	txTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
	tx := &pb.Transaction{Type: pb.Transaction_CHAINLET_NEW, Timestamp: txTime}
	txs := &pb.TransactionBlock{Transactions: []*pb.Transaction{tx}}
	txsPacked, _ := proto.Marshal(txs)

	msg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_REQUEST,
		Payload: txsPacked,
	}
	err := instance.RecvMsg(msg)
	if err != nil {
		t.Fatalf("Failed to handle request: %s", err)
	}
}

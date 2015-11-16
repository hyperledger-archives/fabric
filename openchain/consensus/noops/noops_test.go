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
	"golang.org/x/net/context"
)

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

func TestNoopRequest(t *testing.T) {
	mock := NewMock()
	instance := New(mock)

	txTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
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
}

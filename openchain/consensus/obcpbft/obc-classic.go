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

package obcpbft

import (
	"encoding/base64"
	"fmt"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type obcClassic struct {
	stack consensus.Stack
	pbft  *pbftCore
}

func newObcClassic(id uint64, config *viper.Viper, stack consensus.Stack) *obcClassic {
	op := &obcClassic{stack: stack}
	op.pbft = newPbftCore(id, config, op, stack)
	return op
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcClassic) RecvMsg(ocMsg *pb.OpenchainMessage, senderHandle *pb.PeerID) error {
	if ocMsg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")

		req := &Request{Payload: ocMsg.Payload, ReplicaId: op.pbft.id}
		pbftMsg := &Message{&Message_Request{req}}
		packedPbftMsg, _ := proto.Marshal(pbftMsg)
		op.broadcast(packedPbftMsg)
		op.pbft.request(ocMsg.Payload, op.pbft.id)

		return nil
	}

	if ocMsg.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", ocMsg.Type)
	}

	senderID, err := getValidatorID(senderHandle)
	if err != nil {
		panic("Cannot map sender's PeerID to a valid replica ID")
	}

	op.pbft.receive(ocMsg.Payload, senderID)

	return nil
}

// Close tells us to release resources we are holding
func (op *obcClassic) Close() {
	op.pbft.close()
}

// =============================================================================
// innerStack interface (functions called by pbft-core)
// =============================================================================

// multicast a message to all replicas
func (op *obcClassic) broadcast(msgPayload []byte) {
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPayload,
	}
	op.stack.Broadcast(ocMsg, pb.PeerEndpoint_UNDEFINED)
}

// send a message to a specific replica
func (op *obcClassic) unicast(msgPayload []byte, receiverID uint64) (err error) {
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPayload,
	}
	receiverHandle, err := getValidatorHandle(receiverID)
	if err != nil {
		return
	}
	return op.stack.Unicast(ocMsg, receiverHandle)
}

func (op *obcClassic) sign(msg []byte) ([]byte, error) {
	return op.stack.Sign(msg)
}

func (op *obcClassic) verify(senderID uint64, signature []byte, message []byte) error {
	senderHandle, err := getValidatorHandle(senderID)
	if err != nil {
		return err
	}
	return op.stack.Verify(senderHandle, signature, message)
}

// validate checks whether the request is valid syntactically
// not used in obc-classic at the moment
func (op *obcClassic) validate(txRaw []byte) error {
	return nil
}

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcClassic) execute(txRaw []byte) {
	if err := op.validate(txRaw); err != nil {
		err = fmt.Errorf("Request in transaction did not validate: %s", err)
		logger.Error(err.Error())
		return
	}

	tx := &pb.Transaction{}
	err := proto.Unmarshal(txRaw, tx)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshal transaction: %v", err)
		logger.Error(err.Error())
		return
	}

	txs := []*pb.Transaction{tx}
	txBatchID := base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(txRaw))

	if err := op.stack.BeginTxBatch(txBatchID); err != nil {
		err = fmt.Errorf("Failed to begin transaction %s: %v", txBatchID, err)
		logger.Error(err.Error())
		return
	}

	if _, err := op.stack.ExecTxs(txBatchID, txs); nil != err {
		err = fmt.Errorf("Failed to execute transaction %s: %v", txBatchID, err)
		logger.Error(err.Error())
		if err = op.stack.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}

	if _, err = op.stack.CommitTxBatch(txBatchID, nil); err != nil {
		err = fmt.Errorf("Failed to commit transaction %s to the ledger: %v", txBatchID, err)
		logger.Error(err.Error())
		if err = op.stack.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}
}

// called when a view-change happened in the underlying PBFT
// classic mode pbft does not use this information
func (op *obcClassic) viewChange(curView uint64) {
}

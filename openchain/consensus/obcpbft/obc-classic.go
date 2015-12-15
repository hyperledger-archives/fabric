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
	cpi  consensus.CPI
	pbft *pbftCore
}

func newObcClassic(id uint64, config *viper.Viper, cpi consensus.CPI) *obcClassic {
	op := &obcClassic{cpi: cpi}
	op.pbft = newPbftCore(id, config, op)
	return op
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcClassic) RecvMsg(ocMsg *pb.OpenchainMessage) error {
	if ocMsg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")

		if err := op.verify(ocMsg.Payload); err != nil {
			logger.Warning("Request did not verify: %s", err)
			return err
		}

		op.pbft.request(ocMsg.Payload)

		req := &Request{Payload: ocMsg.Payload}
		msg := &Message{&Message_Request{req}}
		msgRaw, _ := proto.Marshal(msg)
		op.broadcast(msgRaw)

		return nil
	}

	if ocMsg.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", ocMsg.Type)
	}

	pbftMsg := &Message{}
	err := proto.Unmarshal(ocMsg.Payload, pbftMsg)
	if err != nil {
		return err
	}
	if req := pbftMsg.GetRequest(); req != nil {
		op.pbft.request(req.Payload)
	} else {
		op.pbft.receive(ocMsg.Payload)
	}

	return nil
}

// Close tells us to release resources we are holding
func (op *obcClassic) Close() {
	op.pbft.close()
}

// =============================================================================
// innerCPI interface (functions called by pbft-core)
// =============================================================================

// multicast a message to all replicas
func (op *obcClassic) broadcast(msgPayload []byte) {
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPayload,
	}
	op.cpi.Broadcast(ocMsg)
}

// verify checks whether the request is valid
func (op *obcClassic) verify(txRaw []byte) error {
	tx := &pb.Transaction{}
	err := proto.Unmarshal(txRaw, tx)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal transaction: %v", err)
	}
	// TODO verify transaction
	// if _, err := instance.cpi.TransactionPreValidation(...); err != nil {
	//   logger.Warning("Invalid request");
	//   return err
	// }
	return nil
}

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcClassic) execute(txRaw []byte) {
	if err := op.verify(txRaw); err != nil {
		logger.Error("Request in transaction did not verify: %s", err)
		return
	}

	tx := &pb.Transaction{}
	err := proto.Unmarshal(txRaw, tx)
	if err != nil {
		logger.Error("Unable to unmarshal transaction: %v", err)
		return
	}

	txs := []*pb.Transaction{tx}
	txBatchID := base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(txRaw))

	if err := op.cpi.BeginTxBatch(txBatchID); err != nil {
		logger.Error("Failed to begin transaction %s: %v", txBatchID, err)
		return
	}

	_, errs := op.cpi.ExecTXs(txs)
	if errs[len(txs)] != nil {
		logger.Error("Fail to execute transaction %s: %v", txBatchID, errs)
		if err = op.cpi.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}

	if err = op.cpi.CommitTxBatch(txBatchID, txs, nil); err != nil {
		logger.Error("Failed to commit transaction %s to the ledger: %v", txBatchID, err)
		if err = op.cpi.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction %s: %v", txBatchID, err))
		}
		return
	}
}

// viewChange is called when a view-change happened in the underlying pbft
// Classic mode pbft does not use this information.
func (op *obcClassic) viewChange(curView uint64) {
}

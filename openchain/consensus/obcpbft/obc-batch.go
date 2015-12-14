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
	"fmt"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type obcBatch struct {
	cpi        consensus.CPI
	pbft       *pbftCore
	batchSize  int
	batchStore map[string]*Request
}

func newObcBatch(id uint64, config *viper.Viper, cpi consensus.CPI) *obcBatch {
	op := &obcBatch{cpi: cpi}
	op.pbft = newPbftCore(id, config, op)
	op.batchSize = config.GetInt("general.batchSize")
	op.batchStore = make(map[string]*Request)
	return op
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcBatch) RecvMsg(ocMsg *pb.OpenchainMessage) error {
	if ocMsg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")
		// TODO verify transaction
		// if _, err := op.cpi.TransactionPreValidation(...); err != nil {
		//   logger.Warning("Invalid request");
		//   return err
		// }

		req := &Request{Payload: ocMsg.Payload, ReplicaId: op.pbft.id}

		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView { // primary
			err := op.leaderProcReq(req)
			if err != nil {
				return err
			}
		} else { // backup
			msg := &Message{&Message_Request{req}}
			msgRaw, _ := proto.Marshal(msg)
			op.broadcast(msgRaw)
		}
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
		// TODO verify first, we need to be sure about the sender
		switch req.ReplicaId {
		case op.pbft.primary(op.pbft.view):
			// a request sent by the primary; primary should ignore this
			if op.pbft.primary(op.pbft.view) != op.pbft.id {
				op.pbft.request(req.Payload)
			}
		default:
			// a request sent by a backup; backups should ignore this
			if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView {
				err := op.leaderProcReq(req)
				if err != nil {
					return err
				}
			}
		}
	} else {
		op.pbft.receive(ocMsg.Payload)
	}

	return nil
}

// =============================================================================
// innerCPI interface (functions called by pbft-core)
// =============================================================================

// multicast a message to all replicas
func (op *obcBatch) broadcast(msgPayload []byte) {
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPayload,
	}
	op.cpi.Broadcast(ocMsg)
}

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcBatch) execute(tbRaw []byte) {
	tb := &pb.TransactionBlock{}
	err := proto.Unmarshal(tbRaw, tb)
	if err != nil {
		return
	}
	// TODO verify transaction
	// if tx, err = op.cpi.TransactionPreExecution(...); err != nil {
	//   logger.Error("Invalid request");
	// } else {
	// ...
	// }
	// XXX switch to https://github.com/openblockchain/obc-peer/issues/340
	op.cpi.ExecTXs(tb.Transactions)
}

// signal when a view-change happened
func (op *obcBatch) viewChange(curView uint64) {
	// TODO
}

// =============================================================================
// functions specific to batch mode
// =============================================================================

func (op *obcBatch) leaderProcReq(req *Request) error {
	digest := hashReq(req)
	op.batchStore[digest] = req

	if len(op.batchStore) >= op.batchSize {
		// assemble new Request message
		txs := make([]*pb.Transaction, len(op.batchStore))
		var i int
		for d, req := range op.batchStore {
			txs[i] = &pb.Transaction{}
			err := proto.Unmarshal(req.Payload, txs[i])
			if err != nil {
				err = fmt.Errorf("Unable to unpack payload of request %d", i)
				logger.Error("%s", err)
				return err
			}
			i++
			delete(op.batchStore, d) // clean up
		}
		tb := &pb.TransactionBlock{Transactions: txs}
		tbPacked, err := proto.Marshal(tb)
		if err != nil {
			err = fmt.Errorf("Unable to pack transaction block for new batch request")
			logger.Error("%s", err)
			return err
		}
		// process internally
		op.pbft.request(tbPacked)
		// broadcast
		batchReq := &Request{Payload: tbPacked}
		msg := &Message{&Message_Request{batchReq}}
		msgRaw, _ := proto.Marshal(msg)
		op.broadcast(msgRaw)
	}

	return nil
}

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
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type obcBatch struct {
	cpi  consensus.CPI
	pbft *pbftCore

	batchSize        int
	batchStore       map[string]*Request
	batchTimer       *time.Timer
	batchTimerActive bool
	batchTimeout     time.Duration
}

func newObcBatch(id uint64, config *viper.Viper, cpi consensus.CPI) *obcBatch {
	var err error
	op := &obcBatch{cpi: cpi}
	op.pbft = newPbftCore(id, config, op)
	op.batchSize = config.GetInt("general.batchSize")
	op.batchStore = make(map[string]*Request)
	op.batchTimeout, err = time.ParseDuration(config.GetString("general.timeout.batch"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse batch timeout: %s", err))
	}
	// create non-running timer XXX ugly
	op.batchTimer = time.NewTimer(100 * time.Hour)
	op.batchTimer.Stop()
	go op.batchTimerHander()
	return op
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcBatch) RecvMsg(ocMsg *pb.OpenchainMessage) error {
	if ocMsg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")

		if err := op.verify(ocMsg.Payload); err != nil {
			logger.Warning("Request did not verify: %s", err)
			return err
		}

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
		if err = op.verify(req.Payload); err != nil {
			logger.Warning("Request did not verify: %s", err)
			return err
		}

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

// Close tells us to release resources we are holding
func (op *obcBatch) Close() {
	op.pbft.close()
	op.batchTimer.Reset(0)
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

// verify checks whether the request is valid
func (op *obcBatch) verify(txRaw []byte) error {
	// TODO verify transaction
	/* tx := &pb.Transaction{}
	err := proto.Unmarshal(txRaw, tx)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal transaction: %v", err)
	}
	if _, err := instance.cpi.TransactionPreValidation(...); err != nil {
		logger.Warning("Invalid request");
		return err
	} */
	return nil
}

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcBatch) execute(tbRaw []byte) {
	tb := &pb.TransactionBlock{}
	err := proto.Unmarshal(tbRaw, tb)
	if err != nil {
		return
	}

	txs := tb.Transactions
	txBatchID := base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(tbRaw))

	for i, tx := range txs {
		txRaw, _ := proto.Marshal(tx)
		if err = op.verify(txRaw); err != nil {
			logger.Error("Request in transaction %d from batch %s did not verify: %s", i, txBatchID, err)
			return
		}
	}

	if err := op.cpi.BeginTxBatch(txBatchID); err != nil {
		logger.Error("Failed to begin transaction batch %s: %v", txBatchID, err)
		return
	}

	_, errs := op.cpi.ExecTXs(txs)
	if errs[len(txs)] != nil {
		logger.Error("Fail to execute transaction batch %s: %v", txBatchID, errs)
		if err = op.cpi.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction batch %s: %v", txBatchID, err))
		}
		return
	}

	if err = op.cpi.CommitTxBatch(txBatchID, txs, nil); err != nil {
		logger.Error("Failed to commit transaction batch %s to the ledger: %v", txBatchID, err)
		if err = op.cpi.RollbackTxBatch(txBatchID); err != nil {
			panic(fmt.Errorf("Unable to rollback transaction batch %s: %v", txBatchID, err))
		}
		return
	}
}

// signal when a view-change happened
func (op *obcBatch) viewChange(curView uint64) {
	if op.batchTimerActive {
		op.stopBatchTimer()
	}
}

// returns the state hash that corresponds to a specific block in the chain
// if called with no arguments, it returns the latest/temp state hash
func (op *obcBatch) getStateHash(blockNumber ...uint64) (stateHash []byte, err error) {
	if len(blockNumber) == 0 {
		return op.cpi.GetCurrentStateHash()
	}

	block, err := op.cpi.GetBlock(blockNumber[0])
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve block #%v: %s", blockNumber[0], err)
	}
	stateHash, err = block.GetHash()
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve hash for block #%v: %s", blockNumber[0], err)
	}
	return
}

// =============================================================================
// functions specific to batch mode
// =============================================================================

func (op *obcBatch) leaderProcReq(req *Request) error {
	digest := hashReq(req)
	op.batchStore[digest] = req

	if !op.batchTimerActive {
		op.startBatchTimer()
	}

	if len(op.batchStore) >= op.batchSize {
		op.sendBatch()
	}

	return nil
}

func (op *obcBatch) sendBatch() error {
	op.stopBatchTimer()
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

	return nil
}

// allow the primary to send a batch when the timer expires
func (op *obcBatch) batchTimerHander() {
	for {
		select {
		case <-op.batchTimer.C:
			op.pbft.lock.Lock()
			if op.pbft.closed {
				op.pbft.lock.Unlock()
				return
			}
			logger.Info("Replica %d batch timeout expired", op.pbft.id)
			if op.pbft.activeView && (len(op.batchStore) > 0) {
				op.sendBatch()
			}
			op.pbft.lock.Unlock()
		}
	}
}

func (op *obcBatch) startBatchTimer() {
	op.batchTimer.Reset(op.batchTimeout)
	logger.Debug("Replica %d started the batch timer", op.pbft.id)
	op.batchTimerActive = true
}

func (op *obcBatch) stopBatchTimer() {
	op.batchTimer.Stop()
	logger.Debug("Replica %d stopped the batch timer", op.pbft.id)
	op.batchTimerActive = false
	if op.pbft.closed {
		return
	}
loopBatch:
	for {
		select {
		case <-op.batchTimer.C:
		default:
			break loopBatch
		}
	}
}

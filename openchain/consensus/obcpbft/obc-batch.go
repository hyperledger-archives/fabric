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
	"time"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type obcBatch struct {
	cpi              consensus.CPI
	pbft             *pbftCore
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

	txs := tb.Transactions
	_, _ = op.cpi.ExecTXs(txs)

	/* if ledger, err := ledger.GetLedger(); err != nil {
		panic(fmt.Errorf("Fail to get the ledger: %v", err))
	}

	txBatchID := base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(tbRaw))

	if err = ledger.BeginTxBatch(txBatchID); err != nil {
		panic(fmt.Errorf("Fail to begin transactions with the ledger: %v", err))
	}

	hash, errs := op.cpi.ExecTXs(txs)
	// There are n+1 elements of errors in this array. On complete success
	// they'll all be nil. In particular, the last err will be error in
	// producing the hash, if any. That's the only error we do want to check

	if errs[len(txs)] != nil {
		panic(fmt.Errorf("Fail to execute transactions: %v", errs))
	}

	if err = ledger.CommitTxBatch(txBatchID, txs, nil); err != nil {
		ledger.RollbackTxBatch(txBatchID)
		panic(fmt.Errorf("Fail to commit transactions to the ledger: %v", err))
	} */
}

// signal when a view-change happened
func (op *obcBatch) viewChange(curView uint64) {
	if op.batchTimerActive {
		op.stopBatchTimer()
	}
}

// =============================================================================
// functions specific to batch mode
// =============================================================================

// tear down resources opened by newObcBatch
func (op *obcBatch) close() {
	op.batchTimer.Reset(0)
}

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

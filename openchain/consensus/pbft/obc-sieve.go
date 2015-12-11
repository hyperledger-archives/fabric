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
	"encoding/base64"
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/spf13/viper"
)

type ObcSieve struct {
	cpi  consensus.CPI // link to the CPI
	pbft *Plugin

	id            uint64
	view          uint64
	blockNumber   uint64
	currentReq    string
	currentResult []byte
	currentTx     []*pb.Transaction
	reqStore      map[string][]byte
	verifyStore   []*Verify
}

func NewObcSieve(id uint64, config *viper.Viper, cpi consensus.CPI) *ObcSieve {
	op := &ObcSieve{cpi: cpi, id: id}
	op.pbft = NewPbft(id, config, op)
	op.reqStore = make(map[string][]byte)

	return op
}

// Close tears down all resources
func (op *ObcSieve) Close() {
	op.pbft.Close()
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack.  New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *ObcSieve) RecvMsg(msgWrapped *pb.OpenchainMessage) error {
	if msgWrapped.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")
		// TODO verify transaction
		// if _, err := op.cpi.TransactionPreValidation(...); err != nil {
		//   logger.Warning("Invalid request");
		//   return err
		// }
		msg := &SieveMessage{&SieveMessage_Request{msgWrapped.Payload}}
		msgRaw, _ := proto.Marshal(msg)
		op.recvRequest(msgRaw)
		op.Broadcast(msgRaw)
		return nil
	}
	if msgWrapped.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", msgWrapped.Type)
	}

	sieveMsg := &SieveMessage{}
	err := proto.Unmarshal(msgWrapped.Payload, sieveMsg)
	if err != nil {
		return err
	}
	if req := sieveMsg.GetRequest(); req != nil {
		op.recvRequest(req)
	} else if exec := sieveMsg.GetExecute(); exec != nil {
		op.recvExecute(exec)
	} else if verify := sieveMsg.GetVerify(); verify != nil {
		op.recvVerify(verify)
	} else if pbftMsg := sieveMsg.GetPbftMessage(); pbftMsg != nil {
		op.pbft.Receive(pbftMsg)
	}
	return nil
}

// ViewChange is called by the inner pbft to signal when a view change
// happened.
func (op *ObcSieve) ViewChange(curView uint64) {
	op.view = curView
	if op.currentReq != "" {
		op.rollback()
		op.blockNumber -= 1
		op.currentReq = ""
	}
}

// Broadcast is called by the inner pbft to multicast a message to all
// replicas.
func (op *ObcSieve) Broadcast(payload []byte) {
	msg := &SieveMessage{&SieveMessage_PbftMessage{payload}}
	op.broadcast(msg)
}

func (op *ObcSieve) broadcast(msg *SieveMessage) {
	payload, _ := proto.Marshal(msg)
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: payload,
	}
	op.cpi.Broadcast(ocMsg)
}

func (op *ObcSieve) recvRequest(txRaw []byte) {
	if op.pbft.getPrimary(op.view) != op.id {
		return
	}

	// tx := &pb.Transaction{}
	// err := proto.Unmarshal(txRaw, tx)
	// if err != nil {
	// 	return
	// }
	// TODO verify transaction
	// if tx, err = op.cpi.TransactionPreExecution(...); err != nil {
	//   logger.Error("Invalid request");
	// } else {
	// ...
	// }

	// XXX concurrency control

	op.verifyStore = nil

	exec := &Execute{
		View:        op.view,
		BlockNumber: op.blockNumber + 1,
		Request:     txRaw,
		ReplicaId:   op.id,
	}
	op.recvExecute(exec)
	op.broadcast(&SieveMessage{&SieveMessage_Execute{exec}})
}

func (op *ObcSieve) recvExecute(exec *Execute) {
	if exec.View != op.view || op.pbft.getPrimary(op.view) != exec.ReplicaId {
		logger.Debug("Invalid execute from %d", exec.ReplicaId)
		return
	}

	if exec.BlockNumber != op.blockNumber+1 {
		logger.Debug("Invalid block number in execute: expected %d, got %d",
			op.blockNumber+1, exec.BlockNumber)
		return
	}

	if op.currentReq != "" {
		logger.Warning("New execute while waiting for decision")
		return
	}

	op.currentReq = base64.StdEncoding.EncodeToString(exec.Request)
	op.blockNumber += 1

	op.begin()
	tx := &pb.Transaction{}
	proto.Unmarshal(exec.Request, tx)
	op.currentTx = []*pb.Transaction{tx}
	hashes, _ := op.cpi.ExecTXs(op.currentTx)

	op.currentResult = hashes
	verify := &Verify{
		BlockNumber:   exec.BlockNumber,
		RequestDigest: op.currentReq,
		ResultDigest:  op.currentResult,
		ReplicaId:     exec.ReplicaId,
	}
	op.pbft.sign(verify)
	op.recvVerify(verify)
	op.broadcast(&SieveMessage{&SieveMessage_Verify{verify}})
}

func (op *ObcSieve) recvVerify(verify *Verify) {
	if op.pbft.getPrimary(op.view) != op.id {
		return
	}
	if err := op.pbft.verify(verify); err != nil {
		logger.Error("Invalid verify message: %s", err)
		return
	}
	if verify.BlockNumber != op.blockNumber {
		logger.Debug("Invalid verify block number: expected %d, got %d",
			op.blockNumber, verify.BlockNumber)
		return
	}
	if verify.RequestDigest != op.currentReq {
		logger.Info("Invalid request digest")
		return
	}

	for _, v := range op.verifyStore {
		if v.ReplicaId == verify.ReplicaId {
			logger.Info("Duplicate verify from %d", op.id)
			return
		}
	}
	op.verifyStore = append(op.verifyStore, verify)

	if len(op.verifyStore) < 2*op.pbft.f+1 {
		// still have to wait
		return
	}

	dSet, _ := op.verifyDset(op.verifyStore)
	decision := &Decision{
		BlockNumber:   op.blockNumber,
		RequestDigest: op.currentReq,
		Dset:          dSet,
	}
	req, _ := proto.Marshal(decision)
	op.pbft.Request(req)
}

func (op *ObcSieve) verifyDset(inDset []*Verify) (dSet []*Verify, ok bool) {
	sortV := make(map[string][]*Verify)
	for _, v := range inDset {
		s := base64.StdEncoding.EncodeToString(v.ResultDigest)
		sortV[s] = append(sortV[s], v)
	}
	for _, vs := range sortV {
		if len(vs) >= op.pbft.f+1 {
			dSet = vs
			ok = true
			return
		}
	}
	dSet = op.verifyStore
	ok = false
	return
}

// Execute is called by the inner pbft to execute an opaque request,
// which is a totally-ordered `Decision`
func (op *ObcSieve) Execute(raw []byte) {
	decision := &Decision{}
	err := proto.Unmarshal(raw, decision)
	if err != nil {
		return
	}

	if decision.BlockNumber != op.blockNumber {
		logger.Info("out of sync")
		// we're out of sync, or primary is bad
		// XXX recover
		return
	}

	if decision.RequestDigest != op.currentReq {
		logger.Error("Decision request digest does not match execution")
		// XXX view change
		return
	}

	if len(decision.Dset) < op.pbft.f+1 {
		logger.Error("Not enough verifies in decision: need at least %d, got %d",
			op.pbft.f+1, len(decision.Dset))
		// XXX view change
		return
	}

	for _, v := range decision.Dset {
		if err := op.pbft.verify(v); err != nil {
			logger.Error("Invalid decision/verify message: %s", err)
			// XXX view change
			return
		}
	}

	dSet, quorum := op.verifyDset(decision.Dset)
	if !reflect.DeepEqual(dSet, decision.Dset) {
		logger.Error("Invalid verify message")
		// XXX view change
		return
	}

	if !quorum {
		logger.Error("Execute decision: not deterministic")
		op.rollback()
	} else {
		if !reflect.DeepEqual(op.currentResult, dSet[0].ResultDigest) {
			logger.Warning("Decision successful, but our output does not match")
			op.rollback()
			op.blockNumber -= 1
			// XXX now we're out of sync, fetch result from dSet
		} else {
			logger.Debug("Decision successful, committing result")
			if op.commit() != nil {
				op.rollback()
				// we're out of sync
				// XXX fetch result from dSet
				op.blockNumber -= 1
			}
		}
	}

	op.currentReq = ""
}

func (op *ObcSieve) begin() error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Fail to get the ledger: %v", err)
	}
	if err := ledger.BeginTxBatch(op.currentReq); err != nil {
		return fmt.Errorf("Fail to begin transaction with the ledger: %v", err)
	}
	return nil
}

func (op *ObcSieve) rollback() error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Fail to get the ledger: %v", err)
	}

	if err := ledger.RollbackTxBatch(op.currentReq); err != nil {
		return fmt.Errorf("Fail to rollback transaction with the ledger: %v", err)
	}
	return nil
}

func (op *ObcSieve) commit() error {
	ledger, err := ledger.GetLedger()
	if err != nil {
		return fmt.Errorf("Fail to get the ledger: %v", err)
	}

	if err := ledger.CommitTxBatch(op.currentReq, op.currentTx, nil); err != nil {
		return fmt.Errorf("Fail to commit transaction with the ledger: %v", err)
	}
	return nil
}

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
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/spf13/viper"
)

type obcSieve struct {
	cpi  consensus.CPI
	pbft *pbftCore

	id            uint64
	view          uint64
	blockNumber   uint64
	currentReq    string
	currentResult []byte
	currentTx     []*pb.Transaction
	reqStore      map[string][]byte
	verifyStore   []*Verify
}

func newObcSieve(id uint64, config *viper.Viper, cpi consensus.CPI) *obcSieve {
	op := &obcSieve{cpi: cpi, id: id}
	op.pbft = newPbftCore(id, config, op)
	op.reqStore = make(map[string][]byte)

	return op
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcSieve) RecvMsg(ocMsg *pb.OpenchainMessage) error {
	if ocMsg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")
		// TODO verify transaction
		// if _, err := op.cpi.TransactionPreValidation(...); err != nil {
		//   logger.Warning("Invalid request");
		//   return err
		// }

		svMsg := &SieveMessage{&SieveMessage_Request{ocMsg.Payload}}
		svMsgRaw, _ := proto.Marshal(svMsg)
		op.recvRequest(svMsgRaw)
		op.broadcastMsg(svMsg)
		return nil
	}

	if ocMsg.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", ocMsg.Type)
	}

	op.pbft.lock.Lock()
	defer op.pbft.lock.Unlock()

	svMsg := &SieveMessage{}
	err := proto.Unmarshal(ocMsg.Payload, svMsg)
	if err != nil {
		logger.Error("Could not unmarshal sieve message: %v", ocMsg)
		return err
	}
	if req := svMsg.GetRequest(); req != nil {
		op.recvRequest(req)
	} else if exec := svMsg.GetExecute(); exec != nil {
		op.recvExecute(exec)
	} else if verify := svMsg.GetVerify(); verify != nil {
		op.recvVerify(verify)
	} else if pbftMsg := svMsg.GetPbftMessage(); pbftMsg != nil {
		op.pbft.lock.Unlock()
		op.pbft.receive(pbftMsg)
		op.pbft.lock.Lock()
	} else {
		logger.Error("Received invalid sieve message: %v", svMsg)
	}
	return nil
}

// Close tells us to release resources we are holding
func (op *obcSieve) Close() {
	op.pbft.close()
}

// called by pbft-core to multicast a message to all replicas
func (op *obcSieve) broadcast(msgPayload []byte) {
	svMsg := &SieveMessage{&SieveMessage_PbftMessage{msgPayload}}
	op.broadcastMsg(svMsg)
}

// called by pbft-core to signal when a view change happened
func (op *obcSieve) viewChange(curView uint64) {
	logger.Info("Sieve view change")
	op.view = curView
	for idx := range op.pbft.outstandingReqs {
		delete(op.pbft.outstandingReqs, idx)
	}
	op.pbft.stopTimer()
	if op.currentReq != "" {
		logger.Debug("View change rolling back active request blockNo %d, %s", op.blockNumber, op.currentReq)
		op.rollback()
		op.blockNumber--
		op.currentReq = ""
	}
}

func (op *obcSieve) broadcastMsg(svMsg *SieveMessage) {
	msgPayload, _ := proto.Marshal(svMsg)
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPayload,
	}
	op.cpi.Broadcast(ocMsg)
}

func (op *obcSieve) recvRequest(txRaw []byte) {
	if op.pbft.primary(op.view) != op.id {
		logger.Debug("Sieve backup %d ignoring request", op.id)
		return
	}

	logger.Debug("Sieve primary %d received request", op.id)

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

	op.verifyStore = nil

	exec := &Execute{
		View:        op.view,
		BlockNumber: op.blockNumber + 1,
		Request:     txRaw,
		ReplicaId:   op.id,
	}
	logger.Debug("Sieve primary %d broadcasting execute v=%d, blockNo=%d",
		op.id, exec.View, exec.BlockNumber)
	op.recvExecute(exec)
	op.broadcastMsg(&SieveMessage{&SieveMessage_Execute{exec}})
}

func (op *obcSieve) recvExecute(exec *Execute) {
	if exec.View != op.view || op.pbft.primary(op.view) != exec.ReplicaId {
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

	logger.Debug("Sieve replica %d received exec from %d, v=%d, blockNo=%d",
		op.id, exec.ReplicaId, exec.View, exec.BlockNumber)

	op.currentReq = base64.StdEncoding.EncodeToString(exec.Request)
	op.blockNumber++

	op.begin()
	tx := &pb.Transaction{}
	proto.Unmarshal(exec.Request, tx)
	op.currentTx = []*pb.Transaction{tx}
	hashes, _ := op.cpi.ExecTXs(op.currentTx)

	// For simplicity's sake, we use the pbft timer
	op.pbft.startTimer(op.pbft.requestTimeout)

	op.currentResult = hashes
	verify := &Verify{
		BlockNumber:   exec.BlockNumber,
		RequestDigest: op.currentReq,
		ResultDigest:  op.currentResult,
		ReplicaId:     op.id,
	}
	op.pbft.sign(verify)

	logger.Debug("Sieve replica %d sending verify blockNo=%d",
		op.id, verify.BlockNumber)
	op.recvVerify(verify)
	op.broadcastMsg(&SieveMessage{&SieveMessage_Verify{verify}})
}

func (op *obcSieve) recvVerify(verify *Verify) {
	if op.pbft.primary(op.view) != op.id {
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

	logger.Debug("Sieve primary %d received verify from %d, blockNo=%d, result %s",
		op.id, verify.ReplicaId, verify.BlockNumber, verify.ResultDigest)

	for _, v := range op.verifyStore {
		if v.ReplicaId == verify.ReplicaId {
			logger.Info("Duplicate verify from %d", op.id)
			return
		}
	}
	op.verifyStore = append(op.verifyStore, verify)

	if len(op.verifyStore) == 2*op.pbft.f+1 {
		dSet, _ := op.verifyDset(op.verifyStore)
		decision := &Decision{
			BlockNumber:   op.blockNumber,
			RequestDigest: op.currentReq,
			Dset:          dSet,
		}
		req, _ := proto.Marshal(decision)
		op.pbft.lock.Unlock()
		op.pbft.request(req)
		op.pbft.lock.Lock()
	}
}

func (op *obcSieve) verifyDset(inDset []*Verify) (dSet []*Verify, ok bool) {
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

// verify checks whether the request is valid
func (op *obcSieve) verify(txRaw []byte) error {
	decision := &Decision{}
	err := proto.Unmarshal(txRaw, decision)
	if err != nil {
		return err
	}

	if decision.BlockNumber != op.blockNumber {
		return fmt.Errorf("out of sync")
	}

	if decision.RequestDigest != op.currentReq {
		err := fmt.Errorf("Decision request digest does not match execution")
		logger.Error("%s", err)
		op.pbft.sendViewChange()
		return err
	}

	if len(decision.Dset) < op.pbft.f+1 {
		err := fmt.Errorf("Not enough verifies in decision: need at least %d, got %d",
			op.pbft.f+1, len(decision.Dset))
		logger.Error("%s", err)
		op.pbft.sendViewChange()
		return err
	}

	for _, v := range decision.Dset {
		if err := op.pbft.verify(v); err != nil {
			err := fmt.Errorf("Invalid decision/verify message: %s", err)
			logger.Error("%s", err)
			op.pbft.sendViewChange()
			return err
		}
	}

	dSet, _ := op.verifyDset(decision.Dset)
	if !reflect.DeepEqual(dSet, decision.Dset) {
		err := fmt.Errorf("Invalid verify message")
		logger.Error("%s", err)
		op.pbft.sendViewChange()
		return err
	}

	return nil
}

// called by pbft-core to execute an opaque request,
// which is a totally-ordered `Decision`
func (op *obcSieve) execute(raw []byte) {
	if err := op.verify(raw); err != nil {
		if op.pbft.activeView {
			op.pbft.sendViewChange()
		}
		return
	}

	decision := &Decision{}
	proto.Unmarshal(raw, decision)

	dSet, quorum := op.verifyDset(decision.Dset)

	if !quorum {
		logger.Error("Execute decision: not deterministic")
		op.rollback()
	} else {
		if !reflect.DeepEqual(op.currentResult, dSet[0].ResultDigest) {
			logger.Warning("Decision successful, but our output does not match")
			op.rollback()
			op.blockNumber--
			// XXX now we're out of sync, fetch result from dSet
		} else {
			logger.Debug("Decision successful, committing result")
			if op.commit() != nil {
				op.rollback()
				// we're out of sync
				// XXX fetch result from dSet
				op.blockNumber--
			}
		}
	}

	op.currentReq = ""
}

func (op *obcSieve) begin() error {
	if err := op.cpi.BeginTxBatch(op.currentReq); err != nil {
		return fmt.Errorf("Fail to begin transaction: %v", err)
	}
	return nil
}

func (op *obcSieve) rollback() error {
	if err := op.cpi.RollbackTxBatch(op.currentReq); err != nil {
		return fmt.Errorf("Fail to rollback transaction: %v", err)
	}
	return nil
}

func (op *obcSieve) commit() error {
	if err := op.cpi.CommitTxBatch(op.currentReq, op.currentTx, nil); err != nil {
		return fmt.Errorf("Fail to commit transaction: %v", err)
	}
	return nil
}

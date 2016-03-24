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
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/spf13/viper"
)

type obcSieve struct {
	stack consensus.Stack
	pbft  *pbftCore

	startup chan []byte

	id            uint64
	epoch         uint64
	imminentEpoch uint64
	blockNumber   uint64
	currentReq    string
	currentView   uint64
	lowWaterSeqNo uint64

	verifyStore []*Verify

	queuedExec map[uint64]*Execute
	queuedTx   [][]byte

	executor        Executor
	validResultChan chan *validResult
	executeChan     chan *pbftExecute
}

type pbftExecute struct {
	seqNo    uint64
	txRaw    []byte
	execInfo *ExecutionInfo
}

type validResult struct {
	commit  bool
	sieveID []byte
	peerIDs []*pb.PeerID
}

func newObcSieve(id uint64, config *viper.Viper, stack consensus.Stack) *obcSieve {
	op := &obcSieve{stack: stack, id: id}
	op.queuedExec = make(map[uint64]*Execute)
	op.validResultChan = make(chan *validResult)
	op.startup = make(chan []byte)
	op.executor = NewOBCExecutor(config, op, stack)

	logger.Debug("Replica %d obtaining startup information", id)
	startupInfo := <-op.startup
	close(op.startup)

	op.pbft = newPbftCore(id, config, op, startupInfo)

	op.executeChan = make(chan *pbftExecute, op.pbft.L)
	go op.executeThread()

	return op
}

func (op *obcSieve) Startup(seqNo uint64, id []byte) {
	op.startup <- id
}

// moreCorrectThanByzantineQuorum returns the number of replicas that
// have to agree to guarantee that more correct replicas than
// byzantine replicas agree
func (op *obcSieve) moreCorrectThanByzantineQuorum() int {
	return 2*op.pbft.f + 1
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcSieve) RecvMsg(ocMsg *pb.OpenchainMessage, senderHandle *pb.PeerID) error {
	op.pbft.lock()
	defer op.pbft.unlock()

	if ocMsg.Type == pb.OpenchainMessage_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")
		op.broadcastMsg(&SieveMessage{&SieveMessage_Request{ocMsg.Payload}})
		op.recvRequest(ocMsg.Payload)
		return nil
	}

	if ocMsg.Type != pb.OpenchainMessage_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", ocMsg.Type)
	}

	senderID, err := getValidatorID(senderHandle)
	if err != nil {
		panic("Cannot map sender's PeerID to a valid replica ID")
	}

	svMsg := &SieveMessage{}
	err = proto.Unmarshal(ocMsg.Payload, svMsg)
	if err != nil {
		err = fmt.Errorf("Could not unmarshal sieve message: %v", ocMsg)
		logger.Error(err.Error())
		return err
	}
	if req := svMsg.GetRequest(); req != nil {
		op.recvRequest(req)
	} else if exec := svMsg.GetExecute(); exec != nil {
		if senderID != exec.ReplicaId {
			logger.Warning("Sender ID included in message (%v) doesn't match ID corresponding to the receiving stream (%v)", exec.ReplicaId, senderID)
			return nil
		}
		op.recvExecute(exec)
	} else if verify := svMsg.GetVerify(); verify != nil {
		// check for senderID not needed since verify messages are signed and will be verified
		op.recvVerify(verify)
	} else if pbftMsg := svMsg.GetPbftMessage(); pbftMsg != nil {
		op.pbft.unlock()
		op.pbft.receive(pbftMsg, senderID)
		op.pbft.lock()
	} else {
		err = fmt.Errorf("Received invalid sieve message: %v", svMsg)
		logger.Error(err.Error())
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

// send a message to a specific replica
func (op *obcSieve) unicast(msgPayload []byte, receiverID uint64) (err error) {
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

func (op *obcSieve) sign(msg []byte) ([]byte, error) {
	return op.stack.Sign(msg)
}

func (op *obcSieve) verify(senderID uint64, signature []byte, message []byte) error {
	senderHandle, err := getValidatorHandle(senderID)
	if err != nil {
		return err
	}
	return op.stack.Verify(senderHandle, signature, message)
}

// called by pbft-core to signal when a view change happened
func (op *obcSieve) viewChange(newView uint64) {
	logger.Info("Replica %d observing pbft view change to %d", op.id, newView)
	op.queuedTx = nil
	op.imminentEpoch = newView

	for idx := range op.pbft.outstandingReqs {
		delete(op.pbft.outstandingReqs, idx)
	}
	op.pbft.stopTimer()

	if op.pbft.primary(newView) == op.id {
		flush := &Flush{View: newView}
		flush.ReplicaId = op.id
		op.pbft.sign(flush)
		req := &SievePbftMessage{Payload: &SievePbftMessage_Flush{flush}}
		op.invokePbft(req)
	}
}

func (op *obcSieve) broadcastMsg(svMsg *SieveMessage) {
	msgPayload, _ := proto.Marshal(svMsg)
	ocMsg := &pb.OpenchainMessage{
		Type:    pb.OpenchainMessage_CONSENSUS,
		Payload: msgPayload,
	}
	op.stack.Broadcast(ocMsg, pb.PeerEndpoint_UNDEFINED)
}

func (op *obcSieve) invokePbft(msg *SievePbftMessage) {
	raw, _ := proto.Marshal(msg)
	op.pbft.unlock()
	op.pbft.request(raw, op.id)
	op.pbft.lock()
}

func (op *obcSieve) recvRequest(txRaw []byte) {
	if op.pbft.primary(op.epoch) != op.id || !op.pbft.activeView {
		logger.Debug("Sieve backup %d ignoring request", op.id)
		return
	}

	logger.Debug("Sieve primary %d received request", op.id)
	op.queuedTx = append(op.queuedTx, txRaw)

	if op.currentReq == "" {
		op.processRequest()
	}
}

func (op *obcSieve) processRequest() {
	if len(op.queuedTx) == 0 || op.currentReq != "" {
		return
	}

	txRaw := op.queuedTx[0]
	op.queuedTx = op.queuedTx[1:]
	op.verifyStore = nil

	exec := &Execute{
		View:        op.epoch,
		BlockNumber: op.blockNumber + 1,
		Request:     txRaw,
		ReplicaId:   op.id,
	}
	logger.Debug("Sieve primary %d broadcasting execute epoch=%d, blockNo=%d",
		op.id, exec.View, exec.BlockNumber)
	op.broadcastMsg(&SieveMessage{&SieveMessage_Execute{exec}})
	op.recvExecute(exec)
}

func (op *obcSieve) recvExecute(exec *Execute) {
	if !(exec.View >= op.epoch && exec.BlockNumber > op.blockNumber && op.pbft.primary(exec.View) == exec.ReplicaId) {
		logger.Debug("Invalid execute from %d", exec.ReplicaId)
		return
	}

	if _, ok := op.queuedExec[exec.ReplicaId]; !ok {
		op.queuedExec[exec.ReplicaId] = exec
		op.processExecute()
	}
}

func (op *obcSieve) processExecute() {
	if op.currentReq != "" {
		return
	}

	primary := op.pbft.primary(op.epoch)
	exec := op.queuedExec[primary]
	delete(op.queuedExec, primary)

	if exec == nil {
		return
	}

	if !(exec.View == op.epoch && op.pbft.primary(op.epoch) == exec.ReplicaId && op.pbft.activeView) {
		logger.Debug("Invalid execute from %d", exec.ReplicaId)
		return
	}

	if exec.BlockNumber != op.blockNumber+1 {
		logger.Debug("Block block number in execute wrong: expected %d, got %d",
			op.blockNumber, exec.BlockNumber)
		return
	}

	op.currentReq = base64.StdEncoding.EncodeToString(util.ComputeCryptoHash(exec.Request))

	logger.Debug("Sieve replica %d received exec from %d, epoch=%d, blockNo=%d from request=%s",
		op.id, exec.ReplicaId, exec.View, exec.BlockNumber, op.currentReq)

	// With the execution decoupled from the ordering, this sanity check is challenging and introduces a race
	/*
		blockchainSize, _ := op.stack.GetBlockchainSize()
		blockchainSize--
		if op.blockNumber != blockchainSize {
			logger.Critical("Sieve replica %d block number and ledger blockchain size diverged: blockNo=%d, blockchainSize=%d", op.id, op.blockNumber, blockchainSize)
			return
		}
	*/

	op.currentView = exec.View
	op.blockNumber = exec.BlockNumber

	tx := &pb.Transaction{}
	proto.Unmarshal(exec.Request, tx)

	op.executor.Execute(exec.BlockNumber, []*pb.Transaction{tx}, &ExecutionInfo{
		Validate: true,
	})

}

func (op *obcSieve) Checkpoint(seqNo uint64, id []byte) {
	// Nothing to do for this executor callback, as sieve will checkpoint as needed directly
}

func (op *obcSieve) Validate(seqNo uint64, id []byte) (commit bool, correctedID []byte, peerIDs []*pb.PeerID) {

	sidAsBytes, _ := proto.Marshal(&SieveId{
		BlockNumber: seqNo,
		ObcId:       id,
	})

	logger.Debug("Sieve replica %d executed blockNo=%d, request=%s, id=%x, sid=%x", op.id, op.blockNumber, op.currentReq, id, sidAsBytes)

	verify := &Verify{
		View:          op.currentView,
		BlockNumber:   seqNo,
		RequestDigest: op.currentReq,
		ResultDigest:  sidAsBytes,
		ReplicaId:     op.id,
	}
	op.pbft.sign(verify)

	logger.Debug("Sieve replica %d sending verify blockNo=%d",
		op.id, verify.BlockNumber)

	go func() {
		op.pbft.lock()
		defer op.pbft.unlock()
		// for simplicity's sake, we use the pbft timer
		op.pbft.startTimer(op.pbft.requestTimeout)
		op.broadcastMsg(&SieveMessage{&SieveMessage_Verify{verify}})

		op.recvVerify(verify)
	}()

	logger.Debug("Sieve replica %d waiting for result decision on block %d", op.id, seqNo)

	select {
	case result := <-op.validResultChan:

		sid := &SieveId{}
		proto.Unmarshal(result.sieveID, sid)

		if sid.BlockNumber != seqNo {
			logger.Error("Received a valid result for the wrong block number %d, expecting %d, this indicates a bug", sid.BlockNumber, seqNo)
			return false, nil, nil
		}

		if result.commit {
			logger.Debug("Sieve replica %d received a result decision to commit to %x", op.id, result.sieveID)
			if !reflect.DeepEqual(sidAsBytes, result.sieveID) {
				logger.Warning("Decision successful, but our output does not match (%x) vs (%x)", sidAsBytes, result.sieveID)

				logger.Debug("Sieve replica %d decodes decision to obc-snapshot ID of %x", op.id, sid.ObcId)

				return true, sid.ObcId, result.peerIDs
			} else {
				logger.Debug("Sieve replica %d decision successful, committing result", op.id)
				return true, nil, nil
			}
		} else {
			logger.Debug("Sieve replica %d received a result decision to rollback", op.id)
			return false, nil, nil
		}

	case <-op.pbft.closed:
		logger.Warning("Shutdown requested before decision could be made")
		return false, nil, nil
	}

}

func (op *obcSieve) recvVerify(verify *Verify) {
	if op.pbft.primary(op.epoch) != op.id || !op.pbft.activeView {
		return
	}

	logger.Debug("Sieve primary %d received verify from %d, blockNo=%d, result %x",
		op.id, verify.ReplicaId, verify.BlockNumber, verify.ResultDigest)

	if err := op.pbft.verify(verify); err != nil {
		logger.Warning("Invalid verify message: %s", err)
		return
	}
	if verify.View != op.epoch {
		logger.Debug("Invalid verify view: expected %d, got %d",
			op.epoch, verify.View)
		return
	}
	if verify.BlockNumber != op.blockNumber {
		logger.Debug("Invalid verify block number: expected %d, got %d",
			op.blockNumber, verify.BlockNumber)
		return
	}
	if verify.RequestDigest != op.currentReq {
		logger.Debug("Invalid verify: invalid request digest")
		return
	}

	for _, v := range op.verifyStore {
		if v.ReplicaId == verify.ReplicaId {
			logger.Info("Duplicate verify from %d", op.id)
			return
		}
	}
	op.verifyStore = append(op.verifyStore, verify)

	if len(op.verifyStore) == op.moreCorrectThanByzantineQuorum() {
		logger.Debug("Sieve primary %d has enough verify records to make decision", op.id)
		dSet, _ := op.verifyDset(op.verifyStore)
		verifySet := &VerifySet{
			View:          op.epoch,
			BlockNumber:   op.blockNumber,
			RequestDigest: op.currentReq,
			Dset:          dSet,
		}
		verifySet.ReplicaId = op.id
		op.pbft.sign(verifySet)
		req := &SievePbftMessage{Payload: &SievePbftMessage_VerifySet{verifySet}}
		op.invokePbft(req)
		logger.Debug("Sieve primary %d sent request to PBFT for final ordering", op.id)
	} else {
		logger.Debug("Sieve primary %d ignoring request as verify store is size %d instead of %d", op.id, len(op.verifyStore), op.moreCorrectThanByzantineQuorum())
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
	dSet = inDset
	ok = false
	return
}

// validate checks whether the request is valid syntactically
func (op *obcSieve) validate(rawReq []byte) error {
	req := &SievePbftMessage{}
	err := proto.Unmarshal(rawReq, req)
	if err != nil {
		return err
	}

	if vset := req.GetVerifySet(); vset != nil {
		return op.validateVerifySet(vset)
	} else if flush := req.GetFlush(); flush != nil {
		return op.validateFlush(flush)
	} else {
		return fmt.Errorf("Invalid pbft request")
	}
}

func (op *obcSieve) validateVerifySet(vset *VerifySet) error {
	if err := op.pbft.verify(vset); err != nil {
		return err
	}
	if vset.ReplicaId != op.pbft.primary(vset.View) {
		return fmt.Errorf("pbft request from non-primary")
	}

	dups := make(map[uint64]bool)
	for _, v := range vset.Dset {
		if err := op.pbft.verify(v); err != nil {
			logger.Warning("verify-set invalid: %s", err)
			return err
		}
		if dups[v.ReplicaId] {
			err := fmt.Errorf("verify-set invalid: duplicate entry for replica %d", v.ReplicaId)
			logger.Warning("%s", err)
			return err
		}
		dups[v.ReplicaId] = true
	}

	for _, v := range vset.Dset {
		if v.View != vset.View || v.BlockNumber != vset.BlockNumber || v.RequestDigest != vset.RequestDigest {
			err := fmt.Errorf("verify-set invalid: inconsistent verify member")
			logger.Warning("%s", err)
			return err
		}
	}

	if len(vset.Dset) < op.pbft.f+1 {
		err := fmt.Errorf("verify-set invalid: not enough verifies in vset: need at least %d, got %d",
			op.pbft.f+1, len(vset.Dset))
		logger.Error(err.Error())
		return err
	}

	dSet, _ := op.verifyDset(vset.Dset)
	if !reflect.DeepEqual(dSet, vset.Dset) {
		err := fmt.Errorf("verify-set invalid: d-set not coherent: received %v, calculated %v",
			vset.Dset, dSet)
		logger.Error(err.Error())
		return err
	}

	return nil
}

func (op *obcSieve) validateFlush(flush *Flush) error {
	if err := op.pbft.verify(flush); err != nil {
		return err
	}
	if flush.ReplicaId != op.pbft.primary(flush.View) {
		return fmt.Errorf("pbft request from non-primary")
	}

	if flush.View < op.imminentEpoch {
		return fmt.Errorf("flush for wrong epoch: got %d, expected %d", flush.View, op.imminentEpoch)
	}

	return nil
}

// called by pbft-core to execute an opaque request,
// which is a totally-ordered `Decision`
func (op *obcSieve) execute(seqNo uint64, raw []byte, execInfo *ExecutionInfo) {
	for {
		select {
		case op.executeChan <- &pbftExecute{
			seqNo:    seqNo,
			txRaw:    raw,
			execInfo: execInfo,
		}:
			logger.Debug("Seive replica %d successfully queued transaction for sequence number %d", op.id, seqNo)
			return
		default:
			// This will always eventually empty the channel, so this call never blocks permenately
			// it is okay to drop requests if we are lagging because each request contains the state transfer
			// snapshot id
			for tx := range op.executeChan {
				logger.Warning("Seive replica %d ran out of execution buffer space, dropped transaction for sequence number %d", op.id, tx.seqNo)
			}
		}
	}
}

func (op *obcSieve) executeThread() {
	for {
		select {
		case exec := <-op.executeChan:
			op.executeImpl(exec.seqNo, exec.txRaw, exec.execInfo)
		case <-op.pbft.closed:
			logger.Debug("Sieve replica %d requested to stop", op.id)
			return
		}
	}
}

func (op *obcSieve) executeImpl(seqNo uint64, raw []byte, execInfo *ExecutionInfo) {
	op.pbft.lock()
	defer op.pbft.unlock()
	req := &SievePbftMessage{}
	err := proto.Unmarshal(raw, req)
	if err != nil {
		return
	}

	if vset := req.GetVerifySet(); vset != nil {
		op.executeVerifySet(vset, seqNo, execInfo)
	} else if flush := req.GetFlush(); flush != nil {
		op.executeFlush(flush)
	} else {
		logger.Warning("Invalid pbft request")
	}
}

func (op *obcSieve) executeVerifySet(vset *VerifySet, seqNo uint64, execInfo *ExecutionInfo) {
	sync := false

	logger.Debug("Replica %d received verify-set from pbft, view %d, block %d",
		op.id, vset.View, vset.BlockNumber)

	if vset.View != op.epoch {
		logger.Debug("Replica %d ignoring verify-set for wrong epoch: expected %d, got %d",
			op.id, op.epoch, vset.View)
		return
	}

	if vset.BlockNumber < op.blockNumber {
		logger.Debug("Replica %d ignoring verify-set for old block: expected %d, got %d",
			op.id, op.blockNumber, vset.BlockNumber)
		return
	}

	if vset.BlockNumber == op.blockNumber && op.currentReq == "" {
		logger.Debug("Replica %d ignoring verify-set for already committed block",
			op.id)
		return
	}

	if op.currentReq == "" {
		logger.Debug("Replica %d received verify-set without pending execute",
			op.id)
		sync = true
	}

	if vset.BlockNumber != op.blockNumber {
		logger.Debug("Replica %d received verify-set for wrong block: expected %d, got %d",
			op.id, op.blockNumber, vset.BlockNumber)
		sync = true
	}

	if vset.RequestDigest != op.currentReq {
		logger.Debug("Replica %d received verify-set for different execute",
			op.id)
		sync = true
	}

	dSet, shouldCommit := op.verifyDset(vset.Dset)

	if !shouldCommit {
		if !sync {
			logger.Error("Sieve replica %d execute vset: not deterministic", op.id)
			op.validResultChan <- &validResult{
				commit: false,
			}
			op.blockNumber--
		} else {
			logger.Error("Sieve replica %d told to roll back transactions for a block it doesn't have")
		}
	} else {
		var peers []*pb.PeerID
		for _, n := range dSet {
			peer, err := getValidatorHandle(n.ReplicaId)
			if err == nil {
				peers = append(peers, peer)
			}
		}

		decision := dSet[0].ResultDigest

		if !sync {
			logger.Debug("Sieve replica %d arrived at decision %x for block %d", op.id, decision, vset.BlockNumber)

			op.pbft.unlock()
			op.validResultChan <- &validResult{
				commit:  true,
				sieveID: decision,
				peerIDs: peers,
			}
			if execInfo.Checkpoint {
				op.pbft.Checkpoint(seqNo, decision)
			}
			op.pbft.lock()
		} else {
			logger.Debug("Sieve replica %d must sync to decision %x for block %d", op.id, decision, vset.BlockNumber)
			replicas := make([]uint64, len(dSet))
			for i, n := range dSet {
				replicas[i] = n.ReplicaId
			}
			resultSID := &SieveId{}
			proto.Unmarshal(decision, resultSID)

			op.blockNumber = resultSID.BlockNumber

			op.executor.SkipTo(resultSID.BlockNumber, resultSID.ObcId, peers, execInfo)
		}
	}

	if !(!shouldCommit && sync) {
		logger.Debug("Sieve replica %d clearing currentReq state because result decision is being acted on", op.id)
		op.currentReq = ""

		if len(op.queuedTx) > 0 {
			op.processRequest()
		}

		if op.pbft.primary(op.epoch) != op.id {
			op.processExecute()
		}
	}
}

func (op *obcSieve) executeFlush(flush *Flush) {
	logger.Debug("Replica %d received flush from pbft", op.id)
	if flush.View < op.epoch {
		logger.Warning("Replica %d ignoring old flush for epoch %d, we are in epoch %d",
			op.id, flush.View, op.epoch)
		return
	}
	op.epoch = flush.View
	logger.Info("Replica %d advancing epoch to %d", op.id, op.epoch)
	op.queuedTx = nil
	if op.currentReq != "" {
		logger.Info("Replica %d rolling back speculative execution", op.id)
		op.validResultChan <- &validResult{
			commit: false,
		}
		op.blockNumber--
		op.currentReq = ""
	}
}

func (op *obcSieve) validState(seqNo uint64, id []byte, replicas []uint64, execInfo *ExecutionInfo) {
	resultSID := &SieveId{}
	proto.Unmarshal(id, resultSID)
	op.executor.ValidState(resultSID.BlockNumber, resultSID.ObcId, getValidatorHandles(replicas), execInfo)
}

func (op *obcSieve) skipTo(seqNo uint64, id []byte, replicas []uint64, execInfo *ExecutionInfo) {
	// No-op for sieve, as it will handle its own state synchronization
}

func (op *obcSieve) idleChan() <-chan struct{} {
	return op.executor.IdleChan()
}

func (op *obcSieve) getPBFTCore() *pbftCore {
	return op.pbft
}

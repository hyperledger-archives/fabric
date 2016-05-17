/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package obcpbft

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/consensus"
	pb "github.com/hyperledger/fabric/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
	google_protobuf "google/protobuf"
)

type obcBatch struct {
	obcGeneric

	batchSize        int
	batchStore       []*Request
	batchTimer       *time.Timer
	batchTimerActive bool
	batchTimeout     time.Duration

	incomingChan     chan *batchMessage // Queues messages for processing by main thread
	custodyTimerChan chan custodyInfo   // Queues complaints
	execChan         chan *execInfo     // Signals an execution event
	idleChan         chan struct{}      // Used in unit testing to check for idleness

	complainer   *complainer
	deduplicator *deduplicator

	persistForward
}

type custodyInfo struct {
	hash      string
	req       interface{}
	complaint bool
}

type batchMessage struct {
	msg    *pb.Message
	sender *pb.PeerID
}

type execInfo struct {
	seqNo uint64
	raw   []byte
}

func newObcBatch(id uint64, config *viper.Viper, stack consensus.Stack) *obcBatch {
	var err error

	op := &obcBatch{
		obcGeneric: obcGeneric{stack: stack},
	}

	op.persistForward.persistor = stack

	logger.Debug("Replica %d obtaining startup information", id)

	op.pbft = newPbftCore(id, config, op)

	op.batchSize = config.GetInt("general.batchSize")
	op.batchStore = nil
	op.batchTimeout, err = time.ParseDuration(config.GetString("general.timeout.batch"))
	if err != nil {
		panic(fmt.Errorf("Cannot parse batch timeout: %s", err))
	}

	op.incomingChan = make(chan *batchMessage)
	op.custodyTimerChan = make(chan custodyInfo)
	op.execChan = make(chan *execInfo)

	op.complainer = newComplainer(op, op.pbft.requestTimeout, op.pbft.requestTimeout)
	op.deduplicator = newDeduplicator()

	// create non-running timer
	op.batchTimer = time.NewTimer(100 * time.Hour) // XXX ugly
	op.batchTimer.Stop()

	op.idleChan = make(chan struct{})

	go op.main()
	return op
}

// RecvMsg receives both CHAIN_TRANSACTION and CONSENSUS messages from
// the stack. New transaction requests are broadcast to all replicas,
// so that the current primary will receive the request.
func (op *obcBatch) RecvMsg(ocMsg *pb.Message, senderHandle *pb.PeerID) error {
	op.incomingChan <- &batchMessage{
		msg:    ocMsg,
		sender: senderHandle,
	}

	return nil
}

// Complain is necessary to implement complaintHandler
func (op *obcBatch) Complain(hash string, req *Request, primaryFail bool) {
	op.custodyTimerChan <- custodyInfo{hash, req, primaryFail}
}

// Close tells us to release resources we are holding
func (op *obcBatch) Close() {
	op.complainer.Stop()
	op.batchTimer.Reset(0)
	op.pbft.close()
}

func (op *obcBatch) submitToLeader(req *Request) {
	// submit to current leader
	leader := op.pbft.primary(op.pbft.view)
	if leader == op.pbft.id && op.pbft.activeView {
		op.leaderProcReq(req)
	} else {
		op.unicastMsg(&BatchMessage{&BatchMessage_Request{req}}, leader)
	}
}

func (op *obcBatch) broadcastMsg(msg *BatchMessage) {
	msgPayload, _ := proto.Marshal(msg)
	ocMsg := &pb.Message{
		Type:    pb.Message_CONSENSUS,
		Payload: msgPayload,
	}
	op.stack.Broadcast(ocMsg, pb.PeerEndpoint_UNDEFINED)
}

// send a message to a specific replica
func (op *obcBatch) unicastMsg(msg *BatchMessage, receiverID uint64) {
	msgPayload, _ := proto.Marshal(msg)
	ocMsg := &pb.Message{
		Type:    pb.Message_CONSENSUS,
		Payload: msgPayload,
	}
	receiverHandle, err := getValidatorHandle(receiverID)
	if err != nil {
		return

	}
	op.stack.Unicast(ocMsg, receiverHandle)
}

// =============================================================================
// innerStack interface (functions called by pbft-core)
// =============================================================================

// multicast a message to all replicas
func (op *obcBatch) broadcast(msgPayload []byte) {
	op.stack.Broadcast(op.wrapMessage(msgPayload), pb.PeerEndpoint_UNDEFINED)
}

// send a message to a specific replica
func (op *obcBatch) unicast(msgPayload []byte, receiverID uint64) (err error) {
	receiverHandle, err := getValidatorHandle(receiverID)
	if err != nil {
		return
	}
	return op.stack.Unicast(op.wrapMessage(msgPayload), receiverHandle)
}

func (op *obcBatch) sign(msg []byte) ([]byte, error) {
	return op.stack.Sign(msg)
}

// verify message signature
func (op *obcBatch) verify(senderID uint64, signature []byte, message []byte) error {
	senderHandle, err := getValidatorHandle(senderID)
	if err != nil {
		return err
	}
	return op.stack.Verify(senderHandle, signature, message)
}

// validate checks whether the request is valid syntactically
// not used in obc-batch at the moment
func (op *obcBatch) validate(txRaw []byte) error {
	return nil
}

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcBatch) execute(seqNo uint64, raw []byte) {
	op.execChan <- &execInfo{
		seqNo: seqNo,
		raw:   raw,
	}
}

func (op *obcBatch) executeImpl(seqNo uint64, raw []byte) {
	reqs := &RequestBlock{}
	if err := proto.Unmarshal(raw, reqs); err != nil {
		logger.Warning("Batch replica %d could not unmarshal request block: %s", op.pbft.id, err)
		return
	}

	var txs []*pb.Transaction

	for _, req := range reqs.Requests {
		op.complainer.Success(req)

		if !op.deduplicator.Execute(req) {
			logger.Debug("Batch replica %d received exec of stale request from %d via %d",
				op.pbft.id, req.ReplicaId, req.ReplicaId)
			continue
		}

		tx := &pb.Transaction{}
		if err := proto.Unmarshal(req.Payload, tx); err != nil {
			logger.Warning("Batch replica %d could not unmarshal transaction: %s", op.pbft.id, err)
			continue
		}
		txs = append(txs, tx)
	}

	meta, _ := proto.Marshal(&Metadata{seqNo})

	id := []byte("foo")
	op.stack.BeginTxBatch(id)
	result, err := op.stack.ExecTxs(id, txs)
	_ = err    // XXX what to do on error?
	_ = result // XXX what to do with the result?
	_, err = op.stack.CommitTxBatch(id, meta)

	op.pbft.execDone()
}

// signal when a view-change happened
func (op *obcBatch) viewChange(curView uint64) {
	if op.batchTimerActive {
		op.stopBatchTimer()
	}

	reqs := op.complainer.Restart()
	if op.pbft.primary(op.pbft.view) == op.pbft.id {
		for hash, req := range reqs {
			logger.Info("Replica %d queueing request under custody: %s", op.pbft.id, hash)
			op.leaderProcReq(req)
		}
	}
}

// =============================================================================
// functions specific to batch mode
// =============================================================================

func (op *obcBatch) leaderProcReq(req *Request) error {
	// XXX check req sig

	if !op.deduplicator.Request(req) {
		logger.Debug("Batch replica %d received stale request from %d",
			op.pbft.id, req.ReplicaId)
		return nil
	}

	hash := op.complainer.Custody(req)

	logger.Debug("Batch primary %d queueing new request %s", op.pbft.id, hash)
	op.batchStore = append(op.batchStore, req)

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

	reqBlock := &RequestBlock{op.batchStore}
	op.batchStore = nil

	reqsPacked, err := proto.Marshal(reqBlock)
	if err != nil {
		err = fmt.Errorf("Unable to pack block for new batch request")
		logger.Error(err.Error())
		return err
	}

	// process internally
	logger.Info("Creating batch with %d requests", len(reqBlock.Requests))
	op.pbft.request(reqsPacked, op.pbft.id)

	return nil
}

func (op *obcBatch) processMessage(ocMsg *pb.Message, senderHandle *pb.PeerID) error {
	if ocMsg.Type == pb.Message_CHAIN_TRANSACTION {
		now := time.Now()
		req := &Request{
			Timestamp: &google_protobuf.Timestamp{
				Seconds: now.Unix(),
				Nanos:   int32(now.UnixNano() % 1000000000),
			},
			Payload:   ocMsg.Payload,
			ReplicaId: op.pbft.id,
		}
		// XXX sign req
		hash := op.complainer.Custody(req)

		logger.Info("New consensus request received: %s", hash)

		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView { // primary
			err := op.leaderProcReq(req)
			if err != nil {
				return err
			}
		} else { // backup
			batchMsg := &BatchMessage{&BatchMessage_Request{req}}
			op.broadcastMsg(batchMsg)
		}
		return nil
	}

	if ocMsg.Type != pb.Message_CONSENSUS {
		return fmt.Errorf("Unexpected message type: %s", ocMsg.Type)
	}

	batchMsg := &BatchMessage{}
	err := proto.Unmarshal(ocMsg.Payload, batchMsg)
	if err != nil {
		return err
	}

	if req := batchMsg.GetRequest(); req != nil {
		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView {
			err := op.leaderProcReq(req)
			if err != nil {
				return err
			}
		}
	} else if pbftMsg := batchMsg.GetPbftMessage(); pbftMsg != nil {
		senderID, err := getValidatorID(senderHandle) // who sent this?
		if err != nil {
			panic("Cannot map sender's PeerID to a valid replica ID")
		}
		op.pbft.receive(pbftMsg, senderID)
	} else if complaint := batchMsg.GetComplaint(); complaint != nil {
		if op.pbft.primary(op.pbft.view) == op.pbft.id && op.pbft.activeView {
			return op.leaderProcReq(complaint)
		}

		// XXX check req sig
		if !op.deduplicator.IsNew(complaint) {
			logger.Debug("Batch replica %d received stale complaint from %d",
				op.pbft.id, complaint.ReplicaId)
			return nil
		}

		hash := op.complainer.Complaint(complaint)
		logger.Debug("Batch replica %d received complaint %s", op.pbft.id, hash)

		op.submitToLeader(complaint)
	} else {
		err = fmt.Errorf("Unknown request: %+v", batchMsg)
		logger.Error(err.Error())
	}

	return nil
}

// allow the primary to send a batch when the timer expires
func (op *obcBatch) main() {
	for {
		select {
		case <-op.pbft.closed:
			close(op.idleChan)
			return
		case ocMsg := <-op.incomingChan:
			if err := op.processMessage(ocMsg.msg, ocMsg.sender); nil != err {
				logger.Error("Error processing message: %v", err)
			}
		case <-op.batchTimer.C:
			logger.Info("Replica %d batch timer expired", op.pbft.id)
			if op.pbft.activeView && (len(op.batchStore) > 0) {
				op.sendBatch()
			}
		case c := <-op.custodyTimerChan:
			// XXX filter out complaints that are about old requests

			if !c.complaint {
				logger.Warning("Batch replica %d custody expired, complaining: %s", op.pbft.id, c.hash)
				op.broadcastMsg(&BatchMessage{&BatchMessage_Complaint{c.req.(*Request)}})
			} else {
				if op.pbft.activeView {
					logger.Debug("Batch replica %d complaint timeout expired for %s", op.pbft.id, c.hash)
					op.pbft.sendViewChange()
				}
			}
		case execInfo := <-op.execChan:
			op.executeImpl(execInfo.seqNo, execInfo.raw)
		case op.idleChan <- struct{}{}:
			// Only used to detect thread idleness during unit tests
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
	select {
	case <-op.pbft.closed:
		return
	default:
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

// Wraps a payload into a batch message, packs it and wraps it into
// a Fabric message. Called by broadcast before transmission.
func (op *obcBatch) wrapMessage(msgPayload []byte) *pb.Message {
	batchMsg := &BatchMessage{&BatchMessage_PbftMessage{msgPayload}}
	packedBatchMsg, _ := proto.Marshal(batchMsg)
	ocMsg := &pb.Message{
		Type:    pb.Message_CONSENSUS,
		Payload: packedBatchMsg,
	}
	return ocMsg
}

// Retrieve the idle channel, only used for testing
func (op *obcBatch) idleChannel() <-chan struct{} {
	return op.idleChan
}

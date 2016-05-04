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

	"github.com/hyperledger/fabric/consensus"
	pb "github.com/hyperledger/fabric/protos"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
)

type obcBatch struct {
	obcGeneric
	stack consensus.Stack
	pbft  *pbftCore

	batchSize        int
	batchStore       [][]byte
	batchTimer       *time.Timer
	batchTimerActive bool
	batchTimeout     time.Duration

	incomingChan chan *batchMessage // Queues messages for processing by main thread
	idleChan     chan struct{}      // Used in unit testing to check for idleness

	persistForward
}

type batchMessage struct {
	msg    *pb.Message
	sender *pb.PeerID
}

func newObcBatch(id uint64, config *viper.Viper, stack consensus.Stack) *obcBatch {
	var err error

	op := &obcBatch{
		obcGeneric: obcGeneric{stack},
		stack:      stack,
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

// StateUpdate is a signal from the stack that it has fast-forwarded its state
func (op *obcBatch) StateUpdate(seqNo uint64, id []byte) {
	op.pbft.stateUpdate(seqNo, id)
}

// Close tells us to release resources we are holding
func (op *obcBatch) Close() {
	op.pbft.close()
	op.batchTimer.Reset(0)
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
func (op *obcBatch) execute(seqNo uint64, tbRaw []byte) {

	tb := &pb.TransactionBlock{}
	if err := proto.Unmarshal(tbRaw, tb); err != nil {
		return
	}

	meta, _ := proto.Marshal(&Metadata{seqNo})

	id := []byte("foo")
	op.stack.BeginTxBatch(id)
	result, err := op.stack.ExecTxs(id, tb.Transactions)
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
}

// =============================================================================
// functions specific to batch mode
// =============================================================================

func (op *obcBatch) leaderProcReq(req []byte) error {
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

	// assemble new Request message
	var txs []*pb.Transaction
	store := op.batchStore
	op.batchStore = nil
	for _, req := range store {
		tx := &pb.Transaction{}
		err := proto.Unmarshal(req, tx)
		if err != nil {
			err = fmt.Errorf("Unable to unpack payload of request %v", req)
			logger.Error(err.Error())
			continue
		}
		txs = append(txs, tx)
	}
	tb := &pb.TransactionBlock{Transactions: txs}
	tbPacked, err := proto.Marshal(tb)
	if err != nil {
		err = fmt.Errorf("Unable to pack transaction block for new batch request")
		logger.Error(err.Error())
		return err
	}

	// process internally
	op.pbft.request(tbPacked, op.pbft.id)

	return nil
}

func (op *obcBatch) processMessage(ocMsg *pb.Message, senderHandle *pb.PeerID) error {
	if ocMsg.Type == pb.Message_CHAIN_TRANSACTION {
		logger.Info("New consensus request received")

		if (op.pbft.primary(op.pbft.view) == op.pbft.id) && op.pbft.activeView { // primary
			err := op.leaderProcReq(ocMsg.Payload)
			if err != nil {
				return err
			}
		} else { // backup
			batchMsg := &BatchMessage{&BatchMessage_Request{ocMsg.Payload}}
			packedBatchMsg, _ := proto.Marshal(batchMsg)
			ocMsg := &pb.Message{
				Type:    pb.Message_CONSENSUS,
				Payload: packedBatchMsg,
			}
			op.stack.Broadcast(ocMsg, pb.PeerEndpoint_UNDEFINED)
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
	} else {
		err = fmt.Errorf("Unknown request: %+v", req)
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

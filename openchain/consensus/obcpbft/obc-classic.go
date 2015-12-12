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
		// TODO verify transaction
		// if _, err := op.cpi.TransactionPreValidation(...); err != nil {
		//   logger.Warning("Invalid request");
		//   return err
		// }

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

// execute an opaque request which corresponds to an OBC Transaction
func (op *obcClassic) execute(txRaw []byte) {
	tx := &pb.Transaction{}
	err := proto.Unmarshal(txRaw, tx)
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

	op.cpi.ExecTXs([]*pb.Transaction{tx})
}

// signal when a view-change happened
func (op *obcClassic) viewChange(curView uint64) {
	//
}

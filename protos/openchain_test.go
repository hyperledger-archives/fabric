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

package protos

import (
	"testing"

	"github.com/golang/protobuf/proto"
)

func Test_Transaction_CreateNew(t *testing.T) {
	//transaction := new(Transaction)

	transaction := &Transaction{ChainletID: &ChainletID{Url: "Contract001"}}
	//transaction.ContractID = *proto.String("Contract001")
	t.Logf("transaction: %v", transaction)
	data, err := proto.Marshal(transaction)
	if err != nil {
		t.Errorf("Error marshalling transaction: %s", err)
	}
	t.Logf("data = %v", data)

	transcationUnmarshalled := &Transaction{}
	proto.Unmarshal(data, transcationUnmarshalled)
	t.Logf("Unmarshalled transaction := %v", transcationUnmarshalled)
	t.Logf("transcation Function := %s", transcationUnmarshalled.Function)
	// peerConn := NewPeerConnectionFSM("10.10.10.10:30303")

	// err := peerConn.FSM.Event("HELLO")
	// if err != nil {
	// 	t.Error(err)
	// }
	// if peerConn.FSM.Current() != "established" {
	// 	t.Error("Expected to be in establised state")
	// }

	// err = peerConn.FSM.Event("DISCONNECT")
	// if err != nil {
	// 	t.Error(err)
	// }
}

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

func Test_Block_CreateNew(t *testing.T) {

	chaincodePath := "contract_001"
	chaincodeVersion := "0.0.1"
	/*
		input := &pb.ChaincodeInput{Function: "invoke", Args: {"arg1","arg2"}}
		spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG,
			ChaincodeID: &pb.ChaincodeID{Url: chaincodePath, Version: chaincodeVersion}, CtorMsg: input}

		// Build the ChaincodeInvocationSpec message
		chaincodeInvocationSpec := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

		data, err := proto.Marshal(chaincodeInvocationSpec)
	*/
	var data []byte
	transaction := &Transaction{Type: 2, ChaincodeID: &ChaincodeID{Url: chaincodePath, Version: chaincodeVersion}, Payload: data, Uuid: "001"}
	t.Logf("Transaction: %v", transaction)

	block := NewBlock("proposer1", []*Transaction{transaction})
	t.Logf("Block: %v", block)

	data, err := proto.Marshal(block)
	if err != nil {
		t.Errorf("Error marshalling block: %s", err)
	}
	t.Logf("Marshalled data: %v", data)

	// TODO: This doesn't seem like a proper test. Needs to be edited.
	blockUnmarshalled := &Block{}
	proto.Unmarshal(data, blockUnmarshalled)
	t.Logf("Unmarshalled block := %v", blockUnmarshalled)

}

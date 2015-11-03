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
	"fmt"

	"github.com/golang/protobuf/proto"
)

// Bytes returns this transaction as an array of bytes.
func (transaction *Transaction) Bytes() ([]byte, error) {
	data, err := proto.Marshal(transaction)
	if err != nil {
		logger.Error("Error marshalling transaction: %s", err)
		return nil, fmt.Errorf("Could not marshal transaction: %s", err)
	}
	return data, nil
}

// NewTransaction creates a new transaction. It defines the function to call,
// the chainletID on which the function should be called, and the arguments
// string. The arguments could be a string of JSON, but there is no strict
// requirement.
func NewTransaction(chainletID ChainletID, uuid string, function string, args []string) *Transaction {
	transaction := new(Transaction)
	transaction.ChainletID = &chainletID
	transaction.Uuid = uuid
	transaction.Function = function
	transaction.Args = args
	return transaction
}

// NewChainletDeployTransaction is used to deploy chaincode.
func NewChainletDeployTransaction(chainletDeploymentSpec *ChainletDeploymentSpec, uuid string) (*Transaction, error) {
	transaction := new(Transaction)
	transaction.Type = Transaction_CHAINLET_NEW
	transaction.Uuid = uuid
	transaction.ChainletID = chainletDeploymentSpec.ChainletSpec.GetChainletID()
	if chainletDeploymentSpec.ChainletSpec.GetCtorMsg() != nil {
		transaction.Function = chainletDeploymentSpec.ChainletSpec.GetCtorMsg().Function
		transaction.Args = chainletDeploymentSpec.ChainletSpec.GetCtorMsg().Args
	}
	data, err := proto.Marshal(chainletDeploymentSpec)
	if err != nil {
		logger.Error("Error mashalling payload for chaincode deployment: %s", err)
		return nil, fmt.Errorf("Could not marshal payload for chaincode deployment: %s", err)
	}
	transaction.Payload = data
	return transaction, nil
}

// NewChainletDeployTransaction is used to deploy chaincode.
func NewChainletInvokeTransaction(chaincodeInvocation *ChaincodeInvocation, uuid string) (*Transaction, error) {
	transaction := new(Transaction)
	transaction.Type = Transaction_CHAINLET_EXECUTE
	transaction.Uuid = uuid
	transaction.ChainletID = chaincodeInvocation.ChainletSpec.GetChainletID()
	data, err := proto.Marshal(chaincodeInvocation)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal payload for chaincode invocation: %s", err)
	}
	transaction.Payload = data
	return transaction, nil
}

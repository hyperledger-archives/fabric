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
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
)

// NewTransaction creates a new transaction. It defines the funcation to call,
// the contractID on for which the function should be called, and the string
// which will be the arguments. The arguments could be a string of JSON, but
// there is no requirement.
func NewTransaction(chainletID ChainletID, function string, args []string) *Transaction {
	transaction := new(Transaction)
	transaction.ChainletID = &chainletID
	transaction.Function = function
	transaction.Args = args
	return transaction
}

func NewChainletDeployTransaction(chainletDeploymentSpec *ChainletDeploymentSpec) (*Transaction, error) {
	transaction := new(Transaction)
	transaction.Type = Transaction_CHAINLET_NEW
	transaction.ChainletID = chainletDeploymentSpec.ChainletSpec.GetChainletID()
	if chainletDeploymentSpec.ChainletSpec.GetCtorMsg() != nil {
		transaction.Function = chainletDeploymentSpec.ChainletSpec.GetCtorMsg().Function
		transaction.Args = chainletDeploymentSpec.ChainletSpec.GetCtorMsg().Args
	}
	data, err := proto.Marshal(chainletDeploymentSpec)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error creating new chaincode transaction: %s", err))
	}
	transaction.Payload = data
	return transaction, nil
}

// Bytes returns this transaction as an array of bytes
func (transaction *Transaction) Bytes() ([]byte, error) {
	data, err := proto.Marshal(transaction)
	if err != nil {
		//t.Errorf("Error marshalling block: %s", err)
		return nil, errors.New(fmt.Sprintf("Error marshalling block: %s", err))
	}
	return data, nil
}

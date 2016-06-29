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

package main

import (
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

// Simple ledger Retrieval example which demonstrates storing a string in ledger and retrieving the same

// Init callback representing the invocation of a chaincode
func (t *SimpleChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	var key string
	var value string
	var err error

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting a key and value")
	}

	key = args[0]
	value = args[1]

	// Write the state to the ledger
	err = stub.PutState(key, []byte(value))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//Invoke sets key to a value
func (t *SimpleChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {

	var key string
	var value string
	var err error

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting a key and value")
	}

	key = args[0]
	value = args[1]

	// Write the state to the ledger
	err = stub.PutState(key, []byte(value))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Query callback representing the query of a chaincode
func (t *SimpleChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != "query" {
		return nil, errors.New("Invalid query function name. Expecting \"query\"")
	}
	var key string // Entities
	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting the key for query")
	}

	key = args[0]

	// Get the state from the ledger
	value, err := stub.GetState(key)
	if err != nil {
		//fmt.Printf(" Got error %q \n" + err.Error())
		jsonResp := "{\"Error\":\"Failed to get state for " + key + "\"}" + err.Error()
		return nil, errors.New(jsonResp)
	}

	if value == nil {
		jsonResp := "{\"Error\":\"Nil value for " + key + "\"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"Key\":\"" + key + "\",\"Value\":\"" + string(value) + "\"}"
	fmt.Printf("Query Response:%s, isNil?: %v cap:%v \n", jsonResp, (value == nil), cap(value))
	return value, nil
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

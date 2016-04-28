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
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// This chaincode is a test for chaincode invoking another chaincode - invokes chaincode_example02

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

func (t *SimpleChaincode) getChaincodeToCall(stub *shim.ChaincodeStub) (string, error) {
	//This is the hashcode for github.com/hyperledger/fabric/core/example/chaincode/chaincode_example02
	//if the example is modifed this hashcode will change!!
	chainCodeToCall := "5844bc142dcc9e788785e026e22c855957b2c754c912702c58d997dedbc9a042f05d152f6db0fbd7810d95c1b880c210566c9de3093aae0ab76ad2d90e9cfaa5" //with SHA3

	return chainCodeToCall, nil
}

func (t *SimpleChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	var event string // Indicates whether event has happened. Initially 0
	var eventVal int // State of event
	var err error

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	// Initialize the chaincode
	event = args[0]
	eventVal, err = strconv.Atoi(args[1])
	if err != nil {
		return nil, errors.New("Expecting integer value for event status")
	}
	fmt.Printf("eventVal = %d\n", eventVal)

	err = stub.PutState(event, []byte(strconv.Itoa(eventVal)))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// This chaincode invokes another chaincode - chaincode_example02, upon receipt of an event and changes event state
func (t *SimpleChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	var event string // Event entity
	var eventVal int // State of event
	var err error

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	event = args[0]
	eventVal, err = strconv.Atoi(args[1])
	if err != nil {
		return nil, errors.New("Expected integer value for event state change")
	}

	if eventVal != 1 {
		fmt.Printf("Unexpected event. Doing nothing\n")
		return nil, nil
	}

	// Get the chaincode to call from the ledger
	chainCodeToCall, err := t.getChaincodeToCall(stub)
	if err != nil {
		return nil, err
	}

	f := "invoke"
	invokeArgs := []string{"a", "b", "10"}
	response, err := stub.InvokeChaincode(chainCodeToCall, f, invokeArgs)
	if err != nil {
		errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
		fmt.Printf(errStr)
		return nil, errors.New(errStr)
	}

	fmt.Printf("Invoke chaincode successful. Got response %s", string(response))

	// Write the event state back to the ledger
	err = stub.PutState(event, []byte(strconv.Itoa(eventVal)))
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
	var event string // Event entity
	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting entity to query")
	}

	event = args[0]

	// Get the state from the ledger
	eventValbytes, err := stub.GetState(event)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + event + "\"}"
		return nil, errors.New(jsonResp)
	}

	if eventValbytes == nil {
		jsonResp := "{\"Error\":\"Nil value for " + event + "\"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"Name\":\"" + event + "\",\"Amount\":\"" + string(eventValbytes) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)
	return []byte(jsonResp), nil
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

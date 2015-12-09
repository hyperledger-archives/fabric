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

	"github.com/openblockchain/obc-peer/openchain/chaincode/shim"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}


func (t *SimpleChaincode) init(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
        var A, B, C string    // Entities
        var Aval, Bval, Cval int // Asset holdings
        var err error

        if len(args) != 3 {
                return nil, errors.New("Incorrect number of arguments. Expecting 3")
        }

        // Initialize the chaincode
        A = args[0]
        B = args[1]
        C = args[2]
        Aval = 1
        Bval = 1
        Cval = 1

        // Write the state to the ledger
        err = stub.PutState(A, []byte(strconv.Itoa(Aval)))
        if err != nil {
                return nil, err
        }

        err = stub.PutState(B, []byte(strconv.Itoa(Bval)))
        if err != nil {
                return nil, err
        }

        err = stub.PutState(C, []byte(strconv.Itoa(Cval)))
        if err != nil {
                return nil, err
        }

        return nil, nil
}


func (t *SimpleChaincode) invoke(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	var A, B, C string    // Entities
	var method, signee string    // Entities
	var signeeval int // Asset holdings
	var Aval, Bval, Cval int // Asset holdings
	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	method = args[0]
	signee = args[1]

	if method == "sign"{
		var err error
		signeevalbytes, err := stub.GetState(signee)
		if err != nil {
			return nil, errors.New("Failed to get state")
		}
		signeeval, _ = strconv.Atoi(string(signeevalbytes))

		if  signeeval == 1{
			err = stub.PutState(signee, []byte(strconv.Itoa(2)))
			if err != nil {
				return nil, err
			}
			
		}
	}

	A = "A"
	B = "B"
	C = "C"

	Avalbytes, err := stub.GetState(A)
	if err != nil {
		return nil, errors.New("Failed to get state")
	}
	if Avalbytes == nil {
		jsonResp := "{\"Error\":\"Nil amount for " + A + "\"}"
		return nil, errors.New(jsonResp)
	}
	Aval, _ = strconv.Atoi(string(Avalbytes))

	Bvalbytes, err := stub.GetState(B)
	if err != nil {
		return nil, errors.New("Failed to get state")
	}
	if Bvalbytes == nil {
		jsonResp := "{\"Error\":\"Nil amount for " + B + "\"}"
		return nil, errors.New(jsonResp)
	}
	Bval, _ = strconv.Atoi(string(Bvalbytes))

	Cvalbytes, err := stub.GetState(C)
	if err != nil {
		return nil, errors.New("Failed to get state")
	}
	if Cvalbytes == nil {
		jsonResp := "{\"Error\":\"Nil amount for " + C + "\"}"
		return nil, errors.New(jsonResp)
	}
	Cval, _ = strconv.Atoi(string(Cvalbytes))


	if Aval == 2 && Bval == 2 && Cval == 2 {
		//Insert call to bank.deposit here. 	
		fmt.Printf("All 3 signatures obtained. Calling bank.deposit() \n")


	        chaincodeUrl := "github.com/openblockchain/obc-peer/openchain/example/chaincode/bank"
        	chaincodeVersion := "0.0.1"
        	f := "invoke"
        	invokeArgs := []string{"deposit", "a", "10"}
        	response, err := stub.InvokeChaincode(chaincodeUrl, chaincodeVersion, f, invokeArgs)
        	if err != nil {
                	errStr := fmt.Sprintf("Failed to invoke chaincode. Got error: %s", err.Error())
                	fmt.Printf(errStr)
                	return nil, errors.New(errStr)
        	}

        	fmt.Printf("Invoke chaincode successful. Got response %s", string(response))

	}

	fmt.Printf("Aval = %d, Bval = %d, Cval = %d \n", Aval, Bval, Cval)

	return nil, nil
}



// Run callback representing the invocation of a chaincode
// This chaincode will manage two accounts A and B and will transfer X units from A to B upon invoke
func (t *SimpleChaincode) Run(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {

	// Handle different functions
	if function == "init" {
		// Initialize the entities and their asset holdings
		return t.init(stub, args)
	} else if function == "invoke" {
		// Transaction makes payment of X units from A to B
		return t.invoke(stub, args)
	}

	return nil, nil
}

// Query callback representing the query of a chaincode
func (t *SimpleChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != "query" {
		return nil, errors.New("Invalid query function name. Expecting \"query\"")
	}
	var A string // Entities
	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting name of the person to query")
	}

	A = args[0]

	// Get the state from the ledger
	Avalbytes, err := stub.GetState(A)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + A + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		jsonResp := "{\"Error\":\"Nil amount for " + A + "\"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"Name\":\"" + A + "\",\"Sign\":\"" + string(Avalbytes) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)
	return []byte(jsonResp), nil
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

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

/*
This is a system chaincode used to update the validity period on the ledger. It needs to be deployed at genesis time to avoid the need of
a TCert during deployment and It will be invoked by a system component (probably the TCA) who is the source of the validity period value. 

This component will increment the validity period locally and dispatch an invocation transaction for this chaincode passing the new validity 
period as a parameter.

This chaincode is responsible for the verification of the caller's identity, for that we can use an enrolment certificate id or some other
type of verification. For this to work this id (or certificate, public key, etc) needs to be accessible from the chaincode (probably embedded into
the chaincode).

This is the flow for this to work:

1) Obtain some identity id (enrolment certificate id, certificate, public key, etc).
2) Include the information obtained in 1 into this chaincode.
3) Deploy the chaincode
4) Include the chaincode id into the system component that is going to invoke it (possibly the TCA) so it can dispatch an invoke transaction.

*/

type systemChaincode struct {
}

const system_validity_period_key = "system.validity.period"

// Initialize the in the ledger (this needs to be run only once!!!!)
func (t *systemChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	var vp int64 = 0

	// Initialize the validity period in the ledger (this needs to be run only once!!!!)
	err := stub.PutState(system_validity_period_key, []byte(strconv.FormatInt(vp, 10)))
	if err != nil {
		return nil, err
	}
	
	return nil, nil
}

// Transaction updates system validity period on the ledger
func (t *systemChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	// FIXME: this chaincode needs to be executed by an authorized party. In order to guarantee this, two verifications
	// need to be perfomed:
	// 1. The identity of the caller should be available somehow for the chaincode to perform a check.
	// 2. The ability to determine if the chaincode is executed directly or as a part of a nested execution of chaincodes.
	
	// We need to verify identity of caller and that this is not a nested chaincode invocation
	directly_called_by_TCA := true // stub.ValidateCaller(expectedCaller, stub.GetActualCaller()) && stub.StackDepth() == 0
	
	if directly_called_by_TCA {
		if len(args) != 1 {
			return nil, errors.New("Incorrect number of arguments. Expecting 1")
		}
		
		vp := args[0]
		
		err := stub.PutState(system_validity_period_key, []byte(vp))
		if err != nil {
			return nil, err
		}
	}
	
	return nil, nil
}

// Query callback representing the query of a chaincode
func (t *systemChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != "query" {
		return nil, errors.New("Invalid query function name. Expecting \"query\"")
	}
	
	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting 1")
	}
	
	key := args[0]
	
	if key != system_validity_period_key {
		return nil, errors.New("Incorrect key. Expecting " + system_validity_period_key)
	}
	
	// Get the state from the ledger
	vp, err := stub.GetState(key)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + key + "\"}"
		return nil, errors.New(jsonResp)
	}

	if vp == nil {
		jsonResp := "{\"Error\":\"Nil value for " + key + "\"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"Name\":\"" + key + "\",\"Value\":\"" + string(vp) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)
	return []byte(jsonResp), nil
}

func main() {
	err := shim.Start(new(systemChaincode))
	if err != nil {
		fmt.Printf("Error starting System chaincode: %s", err)
	}
}



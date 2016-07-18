/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/chaincode/shim/crypto/attr"
)

// Attributes2State example simple Chaincode implementation
type Attributes2State struct {
}

func (t *Attributes2State) setStateToAttributes(stub *shim.ChaincodeStub, args []string) error {
	attrHandler, err := attr.NewAttributesHandlerImpl(stub)
	if err != nil {
		return err
	}
	for _, att := range args {
		fmt.Println("Writting attribute " + att)
		attVal, err := attrHandler.GetValue(att)
		if err != nil {
			return err
		}
		err = stub.PutState(att, attVal)
		if err != nil {
			return err
		}
	}
	return nil
}

// Init intializes the chaincode by reading the transaction attributes and storing
// the attrbute values in the state
func (t *Attributes2State) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	err := t.setStateToAttributes(stub, args)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Invoke takes two arguements, a key and value, and stores these in the state
func (t *Attributes2State) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function == "delete" {
		return t.delete(stub, args)
	}

	if function != "submit" {
		return nil, errors.New("Invalid invoke function name. Expecting \"submit\"")
	}
	err := t.setStateToAttributes(stub, args)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Deletes an entity from state
func (t *Attributes2State) delete(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	A := args[0]

	valBytes, err := stub.GetState(A)
	if err != nil {
		return nil, err
	}

	if valBytes == nil {
		return nil, errors.New("Entity not found")
	}

	isOk, err := stub.VerifyAttribute(A, valBytes)
	if err != nil {
		return nil, err
	}
	if isOk {
		// Delete the key from the state in ledger
		err = stub.DelState(A)
		if err != nil {
			return nil, errors.New("Failed to delete state")
		}
	}
	return nil, nil
}

// Query callback representing the query of a chaincode
func (t *Attributes2State) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != "read" {
		return nil, errors.New("Invalid query function name. Expecting \"read\"")
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

	jsonResp := "{\"Name\":\"" + A + "\",\"Amount\":\"" + string(Avalbytes) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)
	return Avalbytes, nil
}

func main() {
	err := shim.Start(new(Attributes2State))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}

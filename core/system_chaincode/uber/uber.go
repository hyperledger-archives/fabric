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

package uber

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos"
	sysccapi "github.com/hyperledger/fabric/core/system_chaincode/api"
)

const (
	DEPLOY  = "deploy"
	START   = "start"
	STOP    = "stop"
	UPGRADE = "upgrade"

	GETTRAN = "get-transaction"
)


//errors
type InvalidFunctionErr string
func (f InvalidFunctionErr) Error() string {
    return fmt.Sprintf("invalid function to uber %s", string(f))
}

type InvalidArgsLenErr int
func (i InvalidArgsLenErr) Error() string {
    return fmt.Sprintf("invalid number of argument to uber %d", int(i))
}

type InvalidArgsErr int
func (i InvalidArgsErr) Error() string {
    return fmt.Sprintf("invalid argument (%d) to uber", int(i))
}

type TXExistsErr string
func (t TXExistsErr) Error() string {
    return fmt.Sprintf("transaction exists %s", string(t))
}

type TXNotFoundErr string
func (t TXNotFoundErr) Error() string {
    return fmt.Sprintf("transaction not found %s", string(t))
}

// UberSysCC implements chaincode lifecycle and policies aroud it
type UberSysCC struct {
}

func (t *UberSysCC) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil, nil
}

// Invoke meta transaction to uber with functions "deploy", "start", "stop", "upgrade"
func (t *UberSysCC) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != DEPLOY && function != START && function != STOP && function != UPGRADE {
		return nil, InvalidFunctionErr(function)
	}

	tx,txbytes,err := sysccapi.GetTransaction(args)

	if err != nil {
		return nil, err
	}

	cds := &pb.ChaincodeDeploymentSpec{}
	err = proto.Unmarshal(tx.Payload, cds)
	if err != nil {
		return nil, err
	}

	barr, err := stub.GetState(cds.ChaincodeSpec.ChaincodeID.Name)
	if barr != nil {
		return nil, TXExistsErr(cds.ChaincodeSpec.ChaincodeID.Name)
	}

	err = sysccapi.Deploy(tx)
	
	if err != nil {
		return nil, fmt.Errorf("Error deploying %s: %s", cds.ChaincodeSpec.ChaincodeID.Name, err)
	}

	err = stub.PutState(cds.ChaincodeSpec.ChaincodeID.Name, txbytes)
	if err != nil {
		return nil, fmt.Errorf("PutState failed for %s: %s", cds.ChaincodeSpec.ChaincodeID.Name, err)
	}

	fmt.Printf("Successfully deployed ;-)\n")

	return nil, err
}

// Query callback representing the query of a chaincode
func (t *UberSysCC) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != GETTRAN {
		return nil, InvalidFunctionErr(function)
	}

	if len(args) != 1 {
		return nil, InvalidArgsLenErr(len(args))
	}

	txbytes, _ := stub.GetState(args[0])
	if txbytes == nil {
		return nil, TXNotFoundErr(args[0])
	}

	return []byte(fmt.Sprintf("Found pb.Transaction for %s", args[0])), nil
}

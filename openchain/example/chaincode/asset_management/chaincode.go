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

	"github.com/openblockchain/obc-peer/openchain/chaincode/shim"
)

// AssetManagementChaincode example simple Asset Management Chaincode implementation
// An asset is represented by a string
type AssetManagementChaincode struct {
}

func (t *AssetManagementChaincode) init(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	if len(args) != 0 {
		return nil, errors.New("Incorrect number of arguments. Expecting 0")
	}

	// Create ownership table
	stub.CreateTable("AssetsOwnership", []*shim.ColumnDefinition{
		&shim.ColumnDefinition{"Asset", shim.ColumnDefinition_STRING, true},
		&shim.ColumnDefinition{"Owner", shim.ColumnDefinition_BYTES, false},
	})

	// Set the admin
	// The metadata will contain the certificate of the administrator
	stub.PutState("admin", stub.GetCallerMetadata())

	return nil, nil
}

// Transaction makes payment of X units from A to B
func (t *AssetManagementChaincode) assign(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	asset := args[0]
	owner := []byte(args[1])

	// Verify the identity of the caller
	// Only an administrator can invoker assign
	adminCertificate, err := stub.GetState("admin")
	if err != nil {
		return nil, errors.New("Failed fetching admin identity")
	}

	ok, err := t.isCaller(stub, adminCertificate)
	if err != nil {
		return nil, errors.New("Failed checking caller identity")
	}
	if !ok {
		return nil, errors.New("The caller is not an administrator")
	}

	// Register assignment
	ok, err = stub.InsertRow("AssetsOwnership", shim.Row{
		Columns: []*shim.Column{shim.Column_String_{String_: asset}, shim.Column_Bytes{Bytes: owner}},
	})

	if !ok && err == nil {
		return nil, errors.New("Asset was already assigned.")
	}

	return nil, err
}

// Deletes an entity from state
func (t *AssetManagementChaincode) transfer(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	asset := args[0]
	newOwner := []byte(args[1])

	// Verify the identity of the caller
	// Only the owner can transfer one of his assets

	row, err := stub.GetRow(
		"AssetsOwnership",
		[]*shim.Column{shim.Column_String_{String_: asset}},
	)
	if err != nil {
		return nil, fmt.Errorf("Failed retrieveing asset [%s]: [%s]", asset, err)
	}

	prvOwner := row.Columns[0].GetBytes()

	// Verify ownership
	ok, err := t.isCaller(stub, prvOwner)
	if err != nil {
		return nil, errors.New("Failed checking caller identity")
	}
	if !ok {
		return nil, errors.New("The caller is not the owner of the asset")
	}

	// At this point, the proof of ownership is valid, then register transfer
	stub.DeleteRow(
		"AssetsOwnership",
		[]*shim.Column{shim.Column_String_{String_: asset}},
	)
	stub.InsertRow(
		"AssetsOwnership",
		shim.Row{
			Columns: []*shim.Column{shim.Column_String_{String_: asset}, shim.Column_Bytes{Bytes: newOwner}},
		})

	return nil, nil
}

func (t *AssetManagementChaincode) isCaller(stub *shim.ChaincodeStub, certificate []byte) (bool, error) {
	// Verify \sigma=Sign(certificate.sk, tx.Payload||tx.Binding) against certificate.vk
	// \sigma is in the metadata

	metadata, err := stub.GetCallerMetadata()
	if err != nil {
		return false, errors.New("Failed getting metadata")
	}
	return stub.VerifySignature(
		certificate,
		metadata,
		append(stub.GetPayload(), stub.GetBinding()...),
	)
}

// Run callback representing the invocation of a chaincode
// This chaincode will manage two accounts A and B and will transfer X units from A to B upon invoke
func (t *AssetManagementChaincode) Run(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {

	// Handle different functions
	if function == "init" {
		// Initialize the administrator of this asset management service
		return t.init(stub, args)
	} else if function == "assign" {
		// Assign ownership
		return t.assign(stub, args)
	} else if function == "transfer" {
		// Transfer ownership
		return t.transfer(stub, args)
	}

	return nil, errors.New("Received unknown function invocation")
}

// Query callback representing the query of a chaincode
func (t *AssetManagementChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != "query" {
		return nil, errors.New("Invalid query function name. Expecting \"query\"")
	}

	return nil, errors.New("Not implemented yet!!!")
}

func main() {
	err := shim.Start(new(AssetManagementChaincode))
	if err != nil {
		fmt.Printf("Error starting AssetManagementChaincode: %s", err)
	}
}

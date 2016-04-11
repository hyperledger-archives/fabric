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

	"github.com/op/go-logging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

var myLogger = logging.MustGetLogger("asset_mgm")

// AssetManagementChaincode example simple Asset Management Chaincode implementation
// with access control enforcement at chaincode level.
// Look here for more information on how to implement access control at chaincode level:
// https://github.com/openblockchain/obc-docs/blob/master/tech/application-ACL.md
// An asset is represented by a string
type AssetManagementChaincode struct {
}

func (t *AssetManagementChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	myLogger.Info("[AssetManagementChaincode] Init")
	if len(args) != 0 {
		return nil, errors.New("Incorrect number of arguments. Expecting 0")
	}

	// Create ownership table
	err := stub.CreateTable("AssetsOwnership", []*shim.ColumnDefinition{
		&shim.ColumnDefinition{"Asset", shim.ColumnDefinition_STRING, true},
		&shim.ColumnDefinition{"Owner", shim.ColumnDefinition_BYTES, false},
	})
	if err != nil {
		return nil, errors.New("Failed creating AssetsOnwership table.")
	}

	// Set the admin
	// The metadata will contain the certificate of the administrator
	adminCert, err := stub.GetCallerMetadata()
	if err != nil {
		return nil, errors.New("Failed getting metadata.")
	}
	if len(adminCert) == 0 {
		return nil, errors.New("Invalid admin certificate. Empty.")
	}

	stub.PutState("admin", adminCert)

	return nil, nil
}

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
		return nil, errors.New("Failed checking admin identity")
	}
	if !ok {
		return nil, errors.New("The caller is not an administrator")
	}

	// Register assignment
	myLogger.Debug("New owner of [%s] is [% x]", asset, owner)

	ok, err = stub.InsertRow("AssetsOwnership", shim.Row{
		Columns: []*shim.Column{
			&shim.Column{Value: &shim.Column_String_{String_: asset}},
			&shim.Column{Value: &shim.Column_Bytes{Bytes: owner}}},
	})

	if !ok && err == nil {
		return nil, errors.New("Asset was already assigned.")
	}

	return nil, err
}

func (t *AssetManagementChaincode) transfer(stub *shim.ChaincodeStub, args []string) ([]byte, error) {
	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2")
	}

	asset := args[0]
	newOwner := []byte(args[1])

	// Verify the identity of the caller
	// Only the owner can transfer one of his assets
	var columns []shim.Column
	col1 := shim.Column{Value: &shim.Column_String_{String_: asset}}
	columns = append(columns, col1)

	row, err := stub.GetRow("AssetsOwnership", columns)
	if err != nil {
		return nil, fmt.Errorf("Failed retrieveing asset [%s]: [%s]", asset, err)
	}

	prvOwner := row.Columns[1].GetBytes()
	myLogger.Debug("Previous owener of [%s] is [% x]", asset, prvOwner)
	if len(prvOwner) == 0 {
		return nil, fmt.Errorf("Invalid previous owner. Nil")
	}

	// Verify ownership
	ok, err := t.isCaller(stub, prvOwner)
	if err != nil {
		return nil, errors.New("Failed checking asset owner identity")
	}
	if !ok {
		return nil, errors.New("The caller is not the owner of the asset")
	}

	// At this point, the proof of ownership is valid, then register transfer
	err = stub.DeleteRow(
		"AssetsOwnership",
		[]shim.Column{shim.Column{Value: &shim.Column_String_{String_: asset}}},
	)
	if err != nil {
		return nil, errors.New("Failed deliting row.")
	}

	_, err = stub.InsertRow(
		"AssetsOwnership",
		shim.Row{
			Columns: []*shim.Column{
				&shim.Column{Value: &shim.Column_String_{String_: asset}},
				&shim.Column{Value: &shim.Column_Bytes{Bytes: newOwner}},
			},
		})
	if err != nil {
		return nil, errors.New("Failed inserting row.")
	}

	return nil, nil
}

func (t *AssetManagementChaincode) isCaller(stub *shim.ChaincodeStub, certificate []byte) (bool, error) {
	// In order to enforce access control, we require that the
	// metadata contains the signature under the signing key corresponding
	// to the verification key inside certificate of
	// the payload of the transaction (namely, function name and args) and
	// the transaction binding (to avoid copying attacks)

	// Verify \sigma=Sign(certificate.sk, tx.Payload||tx.Binding) against certificate.vk
	// \sigma is in the metadata

	sigma, err := stub.GetCallerMetadata()
	if err != nil {
		return false, errors.New("Failed getting metadata")
	}
	payload, err := stub.GetPayload()
	if err != nil {
		return false, errors.New("Failed getting payload")
	}
	binding, err := stub.GetBinding()
	if err != nil {
		return false, errors.New("Failed getting binding")
	}

	myLogger.Debug("passed certificate [% x]", certificate)
	myLogger.Debug("passed sigma [% x]", sigma)
	myLogger.Debug("passed payload [% x]", payload)
	myLogger.Debug("passed binding [% x]", binding)

	return stub.VerifySignature(
		certificate,
		sigma,
		append(payload, binding...),
	)
}

// Run callback representing the invocation of a chaincode
func (t *AssetManagementChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {

	// Handle different functions
	if function == "assign" {
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

	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting name of an asset to query")
	}

	// Who is the owner of the asset?
	asset := args[0]

	var columns []shim.Column
	col1 := shim.Column{Value: &shim.Column_String_{String_: asset}}
	columns = append(columns, col1)

	row, err := stub.GetRow("AssetsOwnership", columns)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed retrieveing asset " + asset + ". Error " + err.Error() + ". \"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"Owner\":\"" + string(row.Columns[1].GetBytes()) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)

	return row.Columns[1].GetBytes(), nil
}

func main() {
	err := shim.Start(new(AssetManagementChaincode))
	if err != nil {
		fmt.Printf("Error starting AssetManagementChaincode: %s", err)
	}
}

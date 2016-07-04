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

// Package shim provides APIs for the chaincode to access its state
// variables, transaction context and call other chaincodes.
package shim

import (
	"fmt"

	gp "google/protobuf"

	"github.com/hyperledger/fabric/core/chaincode/shim/crypto/attr"
)

// MockStub is an implementation of ChaincodeStubInterface for unit testing chaincode.
// Use this instead of ChaincodeStub in your chaincode's unit test calls to Init, Query or Invoke.
type MockStub struct {
	// State keeps
	State map[string][]byte
}

// Constructor to initialise the internal State map
func NewMockStub() *MockStub {
	s := new(MockStub)
	s.State = make(map[string][]byte)
	return s
}

func (stub *MockStub) GetState(key string) ([]byte, error) {
	value := stub.State[key]
	fmt.Println("Getting", key, value)
	return value, nil
}

// PutState writes the specified `value` and `key` into the ledger.
func (stub *MockStub) PutState(key string, value []byte) error {
	fmt.Println("Putting", key, value)
	stub.State[key] = value
	return nil
}

// DelState removes the specified `key` and its value from the ledger.
func (stub *MockStub) DelState(key string) error {
	fmt.Println("Deleting", key, stub.State[key])
	delete(stub.State, key)
	return nil
}

// Not implemented
func (stub *MockStub) RangeQueryState(startKey, endKey string) (*StateRangeQueryIterator, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) CreateTable(name string, columnDefinitions []*ColumnDefinition) error {
	return nil
}

// Not implemented
func (stub *MockStub) GetTable(tableName string) (*Table, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) DeleteTable(tableName string) error {
	return nil
}

// Not implemented
func (stub *MockStub) InsertRow(tableName string, row Row) (bool, error) {
	return false, nil
}

// Not implemented
func (stub *MockStub) ReplaceRow(tableName string, row Row) (bool, error) {
	return false, nil
}

// Not implemented
func (stub *MockStub) GetRow(tableName string, key []Column) (Row, error) {
	var r Row
	return r, nil
}

// Not implemented
func (stub *MockStub) GetRows(tableName string, key []Column) (<-chan Row, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) DeleteRow(tableName string, key []Column) error {
	return nil
}

// Not implemented
func (stub *MockStub) InvokeChaincode(chaincodeName string, function string, args []string) ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) QueryChaincode(chaincodeName string, function string, args []string) ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) ReadCertAttribute(attributeName string) ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) VerifyAttribute(attributeName string, attributeValue []byte) (bool, error) {
	return false, nil
}

// Not implemented
func (stub *MockStub) VerifyAttributes(attrs ...*attr.Attribute) (bool, error) {
	return false, nil
}

// Not implemented
func (stub *MockStub) VerifySignature(certificate, signature, message []byte) (bool, error) {
	return false, nil
}

// Not implemented
func (stub *MockStub) GetCallerCertificate() ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) GetCallerMetadata() ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) GetBinding() ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) GetPayload() ([]byte, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) GetTxTimestamp() (*gp.Timestamp, error) {
	return nil, nil
}

// Not implemented
func (stub *MockStub) SetEvent(name string, payload []byte) error {
	return nil
}

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

// Interfaces to allow testing of chaincode apps with mocked up stubs
package shim

import (
	gp "google/protobuf"

	"github.com/hyperledger/fabric/core/chaincode/shim/crypto/attr"
)

// Chaincode interface must be implemented by all chaincodes. The fabric runs
// the transactions by calling these functions as specified.
type Chaincode interface {
	// Init is called during Deploy transaction after the container has been
	// established, allowing the chaincode to initialize its internal data
	Init(stub ChaincodeStubInterface, function string, args []string) ([]byte, error)

	// Invoke is called for every Invoke transactions. The chaincode may change
	// its state variables
	Invoke(stub ChaincodeStubInterface, function string, args []string) ([]byte, error)

	// Query is called for Query transactions. The chaincode may only read
	// (but not modify) its state variables and return the result
	Query(stub ChaincodeStubInterface, function string, args []string) ([]byte, error)
}

// ChaincodeStubInterface is used by deployable chaincode apps to access and modify their ledgers
type ChaincodeStubInterface interface {
	InvokeChaincode(chaincodeName string, function string, args []string) ([]byte, error)
	QueryChaincode(chaincodeName string, function string, args []string) ([]byte, error)

	// GetState returns the byte array value specified by the `key`.
	GetState(key string) ([]byte, error)

	// PutState writes the specified `value` and `key` into the ledger.
	PutState(key string, value []byte) error

	// DelState removes the specified `key` and its value from the ledger.
	DelState(key string) error

	RangeQueryState(startKey, endKey string) (StateRangeQueryIteratorInterface, error)

	CreateTable(name string, columnDefinitions []*ColumnDefinition) error
	GetTable(tableName string) (*Table, error)
	DeleteTable(tableName string) error
	InsertRow(tableName string, row Row) (bool, error)
	ReplaceRow(tableName string, row Row) (bool, error)
	GetRow(tableName string, key []Column) (Row, error)
	GetRows(tableName string, key []Column) (<-chan Row, error)
	DeleteRow(tableName string, key []Column) error

	ReadCertAttribute(attributeName string) ([]byte, error)
	VerifyAttribute(attributeName string, attributeValue []byte) (bool, error)
	VerifyAttributes(attrs ...*attr.Attribute) (bool, error)
	VerifySignature(certificate, signature, message []byte) (bool, error)
	GetCallerCertificate() ([]byte, error)
	GetCallerMetadata() ([]byte, error)
	GetBinding() ([]byte, error)
	GetPayload() ([]byte, error)
	GetTxTimestamp() (*gp.Timestamp, error)
	SetEvent(name string, payload []byte) error
}

type StateRangeQueryIteratorInterface interface {
	HasNext() bool
	Next() (string, []byte, error)
	Close() error
}

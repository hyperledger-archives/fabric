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

package util

import (
	"crypto/sha256"
	// "encoding/hex"
	// "fmt"
	"math"
)

type UTXO struct {
	// Store Store
}

// MakeUTXO
func MakeUTXO() *UTXO {
	utxo := &UTXO{}
	// utxo.Store = store
	return utxo
}

// Key for a transaction
type Key struct {
	TxHashAsHex string
	TxIndex     uint32
}

// GetTransactionHash returns the hash of a transaction
func (u *UTXO) GetTransactionHash(txData []byte) [32]byte {
	firstHash := sha256.Sum256(txData)
	txHash := sha256.Sum256(firstHash[:])
	return txHash
}

// IsCoinbase check if a index is a coinbase transaction
func (u *UTXO) IsCoinbase(index uint32) bool {
	return index == math.MaxUint32
}

// ExecResult is the result of an execution
type ExecResult struct {
	SumCurrentOutputs uint64
	SumPriorOutputs   uint64
	IsCoinbase        bool
}

// Execute runs the transaction, verifying previous outputs
func (u *UTXO) Execute(txData []byte) (*ExecResult, error) {
	// newTX := ParseUTXOBytes(txData)
	// txHash := u.GetTransactionHash(txData)
	execResult := &ExecResult{}
	// _, ok, err := u.Store.GetState(*currKey)	
	// value, ok, err := u.Store.GetState(*keyToPrevOutput)
	// hex := hex.EncodeToString(txHash[:])
	// fmt.Printf("PUT TRAN %s", hex)
	// u.Store.PutTran(hex, txData)
	// u.Store.DelState(*keyToPrevOutput)
	return execResult, nil
}

// Query for a transaction via its hash
func (u *UTXO) Query(txHashHex string) ([]byte, error) {
	//tx, _, err := u.Store.GetTran(txHashHex)
	return nil, nil
}

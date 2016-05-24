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
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
)

const txFileHashes = "./Hashes_for_first_500_transactions_on_testnet3.txt"
const txFile = "./First_500_transactions_base64_encoded_on_testnet3.txt"

const CONSENSUS_SCRIPT_VERIFY_TX = "01000000017d01943c40b7f3d8a00a2d62fa1d560bf739a2368c180615b0a7937c0e883e7c000000006b4830450221008f66d188c664a8088893ea4ddd9689024ea5593877753ecc1e9051ed58c15168022037109f0d06e6068b7447966f751de8474641ad2b15ec37f4a9d159b02af68174012103e208f5403383c77d5832a268c9f71480f6e7bfbdfa44904becacfad66163ea31ffffffff01c8af0000000000001976a91458b7a60f11a904feef35a639b6048de8dd4d9f1c88ac00000000"
const CONSENSUS_SCRIPT_VERIFY_PREVOUT_SCRIPT = "76a914c564c740c6900b93afc9f1bdaef0a9d466adf6ee88ac"

type block struct {
	transactions []string
}

func makeBlock() *block {
	//return &block{transactions: make([]string, 0)}
	return &block{}
}

func (b *block) addTxAsBase64String(txAsString string) error {
	b.transactions = append(b.transactions, txAsString)
	return nil
}

//getTransactionsAsUTXOBytes returns a readonly channel of the transactions within this block as UTXO []byte
func (b *block) getTransactionsAsUTXOBytes() <-chan []byte {
	resChannel := make(chan []byte)
	go func() {
		defer close(resChannel)
		for _, txAsText := range b.transactions {
			data, err := base64.StdEncoding.DecodeString(txAsText)
			if err != nil {
				panic(fmt.Errorf("Could not decode transaction (%s) into bytes use base64 decoding:  %s\n", txAsText, err))
			}
			resChannel <- data
		}
	}()
	return resChannel
}

var blocksFromFile []*block
var once sync.Once

// getBlocks returns the blocks parsed from txFile, but only parses once.
func getBlocks() ([]*block, error) {
	var err error
	once.Do(func() {
		contents, err := ioutil.ReadFile(txFile)
		if err != nil {
			return
		}
		lines := strings.Split(string(contents), string('\n'))
		var currBlock *block
		for _, line := range lines {
			if strings.HasPrefix(line, "Block") {
				currBlock = makeBlock()
				blocksFromFile = append(blocksFromFile, currBlock)
			} else {
				// Trim out the 'Transacion XX:' part
				//currBlock.addTxAsBase64String(strings.Split(line, ": ")[1])
				currBlock.addTxAsBase64String(line)
			}
		}
	})

	return blocksFromFile, err
}

func TestParse_GetBlocksFromFile(t *testing.T) {
	blocks, err := getBlocks()
	if err != nil {
		t.Fatalf("Error getting blocks from tx file: %s", err)
	}
	for index, b := range blocks {
		t.Logf("block %d has len transactions = %d", index, len(b.transactions))
	}
	t.Logf("Number of blocks = %d from file %s", len(blocks), txFile)
}

//TestBlocks_GetTransactionsAsUTXOBytes will range over blocks and then transactions in UTXO bytes form.
func TestBlocks_GetTransactionsAsUTXOBytes(t *testing.T) {
	blocks, err := getBlocks()
	if err != nil {
		t.Fatalf("Error getting blocks from tx file: %s", err)
	}
	// Loop through the blocks and then range over their transactions in UTXO bytes form.
	for index, b := range blocks {
		t.Logf("block %d has len transactions = %d", index, len(b.transactions))
		for txAsUTXOBytes := range b.getTransactionsAsUTXOBytes() {
			//t.Logf("Tx as bytes = %v", txAsUTXOBytes)
			_ = len(txAsUTXOBytes)
		}
	}
}

func TestParse_UTXOTransactionBytes(t *testing.T) {
	blocks, err := getBlocks()
	if err != nil {
		t.Fatalf("Error getting blocks from tx file: %s", err)
	}
	utxo := MakeUTXO(MakeInMemoryStore())
	// Loop through the blocks and then range over their transactions in UTXO bytes form.
	for index, b := range blocks {
		t.Logf("block %d has len transactions = %d", index, len(b.transactions))
		for txAsUTXOBytes := range b.getTransactionsAsUTXOBytes() {

			newTX := ParseUTXOBytes(txAsUTXOBytes)
			t.Logf("Block = %d, txInputCount = %d, outputCount=%d", index, len(newTX.Txin), len(newTX.Txout))

			// Now store the HEX of txHASH
			execResult, err := utxo.Execute(txAsUTXOBytes)
			if err != nil {
				t.Fatalf("Error executing TX:  %s", err)
			}
			if execResult.IsCoinbase == false {
				if execResult.SumCurrentOutputs > execResult.SumPriorOutputs {
					t.Fatalf("sumOfCurrentOutputs > sumOfPriorOutputs: sumOfCurrentOutputs = %d, sumOfPriorOutputs = %d", execResult.SumCurrentOutputs, execResult.SumPriorOutputs)
				}
			}

			txHash := utxo.GetTransactionHash(txAsUTXOBytes)
			retrievedTx, err := utxo.Query(hex.EncodeToString(txHash[:]))
			if err != nil {
				t.Fatalf("Error querying utxo: %s", err)
			}
			if !bytes.Equal(txAsUTXOBytes, retrievedTx) {
				t.Fatal("Expected TX to be equal. ")
			}
		}
	}

}

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

package trie

import (
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"
	ledgerUtil "github.com/hyperledger/fabric/core/ledger/util"
	"github.com/hyperledger/fabric/core/util"
)

type trieNode struct {
	trieKey              *trieKey
	value                []byte
	childrenCryptoHashes map[int][]byte

	valueUpdated                bool
	childrenCryptoHashesUpdated map[int]bool
	markedForDeletion           bool
}

func newTrieNode(key *trieKey, value []byte, updated bool) *trieNode {
	return &trieNode{
		trieKey:              key,
		value:                value,
		childrenCryptoHashes: make(map[int][]byte),

		valueUpdated:                updated,
		childrenCryptoHashesUpdated: make(map[int]bool),
	}
}

func (trieNode *trieNode) getLevel() int {
	return trieNode.trieKey.getLevel()
}

func (trieNode *trieNode) isRootNode() bool {
	return trieNode.trieKey.isRootKey()
}

func (trieNode *trieNode) setChildCryptoHash(index int, childCryptoHash []byte) {
	if index >= trieKeyEncoderImpl.getMaxTrieWidth() {
		panic(fmt.Errorf("Index for child crypto-hash cannot be greater than [%d]. Tried to access index value [%d]", trieKeyEncoderImpl.getMaxTrieWidth(), index))
	}
	if childCryptoHash != nil {
		trieNode.childrenCryptoHashes[index] = childCryptoHash
	}
	trieNode.childrenCryptoHashesUpdated[index] = true
}

func (trieNode *trieNode) getParentTrieKey() *trieKey {
	return trieNode.trieKey.getParentTrieKey()
}

func (trieNode *trieNode) getParentLevel() int {
	return trieNode.trieKey.getParentLevel()
}

func (trieNode *trieNode) getIndexInParent() int {
	return trieNode.trieKey.getIndexInParent()
}

func (trieNode *trieNode) mergeMissingAttributesFrom(dbTrieNode *trieNode) {
	stateTrieLogger.Debug("Enter mergeMissingAttributesFrom() baseNode=[%s], mergeNode=[%s]", trieNode, dbTrieNode)
	if !trieNode.valueUpdated {
		trieNode.value = dbTrieNode.value
	}
	for k, v := range dbTrieNode.childrenCryptoHashes {
		if !trieNode.childrenCryptoHashesUpdated[k] {
			trieNode.childrenCryptoHashes[k] = v
		}
	}
	stateTrieLogger.Debug("Exit mergeMissingAttributesFrom() mergedNode=[%s]", trieNode)
}

func (trieNode *trieNode) computeCryptoHash() []byte {
	stateTrieLogger.Debug("Enter computeCryptoHash() for trieNode [%s]", trieNode)
	var cryptoHashContent []byte
	if trieNode.containsValue() {
		stateTrieLogger.Debug("Adding value to hash computation for trieNode [%s]", trieNode)
		key := trieNode.trieKey.getEncodedBytes()
		cryptoHashContent = append(cryptoHashContent, proto.EncodeVarint(uint64(len(key)))...)
		cryptoHashContent = append(cryptoHashContent, key...)
		cryptoHashContent = append(cryptoHashContent, trieNode.value...)
	}

	sortedChildrenIndexes := trieNode.getSortedChildrenIndex()
	for _, index := range sortedChildrenIndexes {
		childCryptoHash := trieNode.childrenCryptoHashes[index]
		stateTrieLogger.Debug("Adding hash [%#v] for child number [%d] to hash computation for trieNode [%s]", childCryptoHash, index, trieNode)
		cryptoHashContent = append(cryptoHashContent, childCryptoHash...)
	}

	if cryptoHashContent == nil {
		// node has no associated value and no associated children.
		stateTrieLogger.Debug("Returning nil as hash for trieNode = [%s]. Also, marking this key for deletion.", trieNode)
		trieNode.markedForDeletion = true
		return nil
	}

	if !trieNode.containsValue() && trieNode.getNumChildren() == 1 {
		// node has no associated value and has a single child. Propagate the child hash up
		stateTrieLogger.Debug("Returning hash as of a single child for trieKey = [%s]", trieNode.trieKey)
		return cryptoHashContent
	}

	stateTrieLogger.Debug("Recomputing hash for trieKey = [%s]", trieNode)
	return util.ComputeCryptoHash(cryptoHashContent)
}

func (trieNode *trieNode) containsValue() bool {
	if trieNode.isRootNode() {
		return false
	}
	return ledgerUtil.NotNil(trieNode.value)
}

func (trieNode *trieNode) marshal() ([]byte, error) {
	buffer := proto.NewBuffer([]byte{})

	// write value
	err := buffer.EncodeRawBytes(trieNode.value)
	if err != nil {
		return nil, err
	}

	numCryptoHashes := trieNode.getNumChildren()

	//write number of crypto-hashes
	err = buffer.EncodeVarint(uint64(numCryptoHashes))
	if err != nil {
		return nil, err
	}

	if numCryptoHashes == 0 {
		return buffer.Bytes(), nil
	}

	for i, cryptoHash := range trieNode.childrenCryptoHashes {
		//write crypto-hash Index
		err = buffer.EncodeVarint(uint64(i))
		if err != nil {
			return nil, err
		}
		// write crypto-hash
		err = buffer.EncodeRawBytes(cryptoHash)
		if err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func unmarshalTrieNode(key *trieKey, serializedContent []byte) (*trieNode, error) {
	trieNode := newTrieNode(key, nil, false)
	buffer := proto.NewBuffer(serializedContent)
	value, err := buffer.DecodeRawBytes(false)
	if err != nil {
		return nil, err
	}
	trieNode.value = value

	numCryptoHashes, err := buffer.DecodeVarint()
	if err != nil {
		return nil, err
	}

	for i := uint64(0); i < numCryptoHashes; i++ {
		index, err := buffer.DecodeVarint()
		if err != nil {
			return nil, err
		}
		cryptoHash, err := buffer.DecodeRawBytes(false)
		if err != nil {
			return nil, err
		}
		trieNode.childrenCryptoHashes[int(index)] = cryptoHash
	}
	return trieNode, nil
}

func unmarshalTrieNodeValue(serializedContent []byte) []byte {
	buffer := proto.NewBuffer(serializedContent)
	value, err := buffer.DecodeRawBytes(false)
	if err != nil {
		panic(fmt.Errorf("This error is not excpected: %s", err))
	}
	return value
}

func (trieNode *trieNode) String() string {
	return fmt.Sprintf("trieKey=[%s], value=[%#v], Num children hashes=[%#v]",
		trieNode.trieKey, trieNode.value, trieNode.getNumChildren())
}

func (trieNode *trieNode) getNumChildren() int {
	return len(trieNode.childrenCryptoHashes)
}

func (trieNode *trieNode) getSortedChildrenIndex() []int {
	keys := make([]int, trieNode.getNumChildren())
	i := 0
	for k := range trieNode.childrenCryptoHashes {
		keys[i] = k
		i++
	}
	sort.Ints(keys)
	return keys
}

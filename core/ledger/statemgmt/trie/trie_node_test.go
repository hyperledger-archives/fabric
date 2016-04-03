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
	"github.com/hyperledger/fabric/core/ledger/testutil"
	"testing"
)

func TestTrieNode_MarshalUnmarshal_NoValue_NoChildren(t *testing.T) {
	testTrieNodeMarshalUnmarshal(
		newTrieNode(newTrieKey("chaincodeID", "key"),
			[]byte{},
			false),
		t)
}

func TestTrieNode_MarshalUnmarshal_WithValue(t *testing.T) {
	testTrieNodeMarshalUnmarshal(
		newTrieNode(newTrieKey("chaincodeID", "key"),
			[]byte("Hello!"),
			false),
		t)
}

func TestTrieNode_MarshalUnmarshal_WithChildren(t *testing.T) {
	trieNode := newTrieNode(newTrieKey("chaincodeID", "key"), []byte("Hello!"), false)
	trieNode.setChildCryptoHash(0, []byte("crypto-hash-for-test-0"))
	trieNode.setChildCryptoHash(15, []byte("crypto-hash-for-test-15"))
	testTrieNodeMarshalUnmarshal(trieNode, t)
}

func TestTrieNode_MergeAttributes(t *testing.T) {
	trieNode := newTrieNode(newTrieKey("chaincodeID", "key"), []byte("newValue!"), true)
	trieNode.setChildCryptoHash(0, []byte("crypto-hash-for-test-0"))
	trieNode.setChildCryptoHash(5, []byte("crypto-hash-for-test-5"))

	existingTrieNode := newTrieNode(newTrieKey("chaincodeID", "key"), []byte("existingValue"), false)
	existingTrieNode.setChildCryptoHash(5, []byte("crypto-hash-for-test-5-existing"))
	existingTrieNode.setChildCryptoHash(10, []byte("crypto-hash-for-test-10-existing"))

	trieNode.mergeMissingAttributesFrom(existingTrieNode)
	testutil.AssertEquals(t, trieNode.value, []byte("newValue!"))
	testutil.AssertEquals(t, trieNode.childrenCryptoHashes[0], []byte("crypto-hash-for-test-0"))
	testutil.AssertEquals(t, trieNode.childrenCryptoHashes[5], []byte("crypto-hash-for-test-5"))
	testutil.AssertEquals(t, trieNode.childrenCryptoHashes[10], []byte("crypto-hash-for-test-10-existing"))
}

func TestTrieNode_ComputeCryptoHash_NoValue_NoChild(t *testing.T) {
	trieNode := newTrieNode(newTrieKey("chaincodeID", "key"), nil, false)
	hash := trieNode.computeCryptoHash()
	testutil.AssertEquals(t, hash, nil)
}

func TestTrieNode_ComputeCryptoHash_NoValue_SingleChild(t *testing.T) {
	trieNode := newTrieNode(newTrieKey("chaincodeID", "key"), nil, false)
	singleChildCryptoHash := []byte("childCryptoHash-0")
	trieNode.setChildCryptoHash(0, singleChildCryptoHash)
	hash := trieNode.computeCryptoHash()
	testutil.AssertEquals(t, hash, singleChildCryptoHash)
}

func TestTrieNode_ComputeCryptoHash_NoValue_ManyChildren(t *testing.T) {
	trieKey := newTrieKey("chaincodeID", "key")
	child_0_CryptoHash := []byte("childCryptoHash-0")
	child_5_CryptoHash := []byte("childCryptoHash-5")
	child_15_CryptoHash := []byte("childCryptoHash-15")

	trieNode := newTrieNode(trieKey, nil, false)
	trieNode.setChildCryptoHash(0, child_0_CryptoHash)
	trieNode.setChildCryptoHash(5, child_5_CryptoHash)
	trieNode.setChildCryptoHash(15, child_15_CryptoHash)
	hash := trieNode.computeCryptoHash()
	expectedHashContent := expectedCryptoHashForTest(nil, nil, child_0_CryptoHash, child_5_CryptoHash, child_15_CryptoHash)
	testutil.AssertEquals(t, hash, expectedHashContent)
}

func TestTrieNode_ComputeCryptoHash_WithValue_NoChild(t *testing.T) {
	trieKey := newTrieKey("chaincodeID", "key")
	value := []byte("testValue")

	trieNode := newTrieNode(trieKey, value, false)
	hash := trieNode.computeCryptoHash()
	expectedHash := expectedCryptoHashForTest(trieKey, value)
	testutil.AssertEquals(t, hash, expectedHash)
}

func TestTrieNode_ComputeCryptoHash_WithValue_SingleChild(t *testing.T) {
	trieKey := newTrieKey("chaincodeID", "key")
	value := []byte("testValue")
	child_0_CryptoHash := []byte("childCryptoHash-0")

	trieNode := newTrieNode(trieKey, value, false)
	trieNode.setChildCryptoHash(0, child_0_CryptoHash)
	hash := trieNode.computeCryptoHash()
	expectedHash := expectedCryptoHashForTest(trieKey, value, child_0_CryptoHash)
	testutil.AssertEquals(t, hash, expectedHash)
}

func TestTrieNode_ComputeCryptoHash_WithValue_ManyChildren(t *testing.T) {
	trieKey := newTrieKey("chaincodeID", "key")
	value := []byte("testValue")
	child_0_CryptoHash := []byte("childCryptoHash-0")
	child_5_CryptoHash := []byte("childCryptoHash-5")
	child_15_CryptoHash := []byte("childCryptoHash-15")

	trieNode := newTrieNode(trieKey, value, false)
	trieNode.setChildCryptoHash(0, child_0_CryptoHash)
	trieNode.setChildCryptoHash(5, child_5_CryptoHash)
	trieNode.setChildCryptoHash(15, child_15_CryptoHash)
	hash := trieNode.computeCryptoHash()

	expectedHash := expectedCryptoHashForTest(trieKey, value, child_0_CryptoHash, child_5_CryptoHash, child_15_CryptoHash)
	testutil.AssertEquals(t, hash, expectedHash)
}

func testTrieNodeMarshalUnmarshal(trieNode *trieNode, t *testing.T) {
	trieNodeTestWrapper := &trieNodeTestWrapper{trieNode, t}
	serializedContent := trieNodeTestWrapper.marshal()
	trieNodeFromUnmarshal := trieNodeTestWrapper.unmarshal(trieNode.trieKey, serializedContent)
	testutil.AssertEquals(t, trieNodeFromUnmarshal.trieKey, trieNode.trieKey)
	testutil.AssertEquals(t, trieNodeFromUnmarshal.value, trieNode.value)
	testutil.AssertEquals(t, trieNodeFromUnmarshal.childrenCryptoHashes, trieNode.childrenCryptoHashes)
	testutil.AssertEquals(t, trieNodeFromUnmarshal.getNumChildren(), trieNode.getNumChildren())
}

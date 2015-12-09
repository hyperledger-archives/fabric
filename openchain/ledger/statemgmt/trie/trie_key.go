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
	"bytes"
	"fmt"
)

type trieKeyEncoderInterface interface {
	encodeTrieKey(originalBytes []byte) trieKeyInterface
	getMaxTrieWidth() int
	decodeTrieKey(encodedBytes []byte) trieKeyInterface
}

type trieKeyInterface interface {
	getLevel() int
	getParentTrieKey() trieKeyInterface
	getIndexInParent() int
	getEncodedBytes() []byte
}

var trieKeyEncoderImpl = newByteTrieKeyEncoder()
var stateKeyDelimiter = []byte{0x00}
var rootTrieKey = []byte{}
var rootTrieKeyStr = string(rootTrieKey)

type trieKey struct {
	trieKeyImpl trieKeyInterface
}

func newTrieKey(chaincodeID string, key string) *trieKey {
	fullKey := []byte(chaincodeID)
	fullKey = append(fullKey, stateKeyDelimiter...)
	keybytes := ([]byte(key))
	fullKey = append(fullKey, keybytes...)
	return &trieKey{trieKeyEncoderImpl.encodeTrieKey(fullKey)}
}

func (key *trieKey) getEncodedBytes() []byte {
	return key.trieKeyImpl.getEncodedBytes()
}

func (key *trieKey) getLevel() int {
	return key.trieKeyImpl.getLevel()
}

func (key *trieKey) getIndexInParent() int {
	if key.isRootKey() {
		panic(fmt.Errorf("Parent for Trie root shoould not be asked for"))
	}
	return key.trieKeyImpl.getIndexInParent()
}

func (key *trieKey) getParentTrieKey() *trieKey {
	if key.isRootKey() {
		panic(fmt.Errorf("Parent for Trie root shoould not be asked for"))
	}
	return &trieKey{key.trieKeyImpl.getParentTrieKey()}
}

func (key *trieKey) getEncodedBytesAsStr() string {
	return string(key.trieKeyImpl.getEncodedBytes())
}

func (key *trieKey) isRootKey() bool {
	return len(key.getEncodedBytes()) == 0
}

func (key *trieKey) getParentLevel() int {
	if key.isRootKey() {
		panic(fmt.Errorf("Parent for Trie root shoould not be asked for"))
	}
	return key.getLevel() - 1
}

func (key *trieKey) assertIsChildOf(parentTrieKey *trieKey) {
	if !bytes.Equal(key.getParentTrieKey().getEncodedBytes(), parentTrieKey.getEncodedBytes()) {
		panic(fmt.Errorf("trie key [%s] is not a child of trie key [%s]", key, parentTrieKey))
	}
}

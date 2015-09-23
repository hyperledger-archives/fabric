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
	"os"
	"testing"
)

func TestNewTrie(t *testing.T) {
	//var e error
	trieDbPath := "/tmp/openchain_test_trie.db"
	_, err := NewTrie(trieDbPath)
	if err != nil {
		t.Logf("Failed to create trie: %s", err)
		t.Fail()
	}
	t.Logf("Opended trie = %s", trieDbPath)
	defer os.Remove(trieDbPath)
}

func TestNewTrie_AlreadyOpenFail(t *testing.T) {
	trieDbPath := "/tmp/openchain_test_trie.db"
	trie, err := NewTrie(trieDbPath)
	if err != nil {
		t.Logf("Failed to create trie: %s", err)
		t.Fail()
	}
	t.Logf("Opended trie = %s", trie.DB.Path)
	defer os.Remove(trie.DB.Path)

	_, err = NewTrie(trieDbPath)
	defer os.Remove(trieDbPath)
	if err == nil {
		t.Logf("Expected timeout trying create trie, failing this test.")
		t.Fail()
	} else {
		t.Logf("Failed to open trie as expected trie with path = %s", trieDbPath)
	}
}

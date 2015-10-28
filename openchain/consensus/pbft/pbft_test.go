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

package pbft

import "testing"
import "github.com/openblockchain/obc-peer/openchain/consensus/helper"

// Runs when the package is loaded.
func TestGetParam(t *testing.T) {
	// Create new algorithm instance.
	helperInstance := helper.New()
	instance := New(helperInstance)
	helperInstance.SetConsenter(instance)
	// For a key that exists.
	key := "general.name"
	// Expected value.
	realVal := "pbft"
	// Read key.
	testVal, err := instance.GetParam(key)
	// Error should be nil, since the key exists.
	if err != nil {
		t.Fatalf("Error when retrieving value for existing key %s: %s", key, err)
	}
	// Values should match.
	if testVal != realVal {
		t.Fatalf("Expected value %s for key %s, got %s instead.", realVal, key, testVal)
	}
	// Read key.
	key = "non.existing.key"
	_, err = instance.GetParam(key)
	// Error should not be nil, since the key does not exist.
	if err == nil {
		t.Fatal("Expected error since retrieving value for non-existing key, got nil instead.")
	}
}

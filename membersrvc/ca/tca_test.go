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

package ca

import (
	"os"
	"testing"

	"github.com/hyperledger/fabric/core/crypto"
)

func TestNewTCA(t *testing.T) {

	//init the crypto layer
	if err := crypto.Init(); err != nil {
		t.Errorf("Failed initializing the crypto layer [%s]", err)
	}

	//initialize logging to avoid panics in the current code
	LogInit(os.Stdout, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca := NewECA()
	if eca == nil {
		t.Fatal("Could not create a new ECA")
	}

	tca := NewTCA(eca)
	if tca == nil {
		t.Fatal("Could not create a new TCA")
	}

	if tca.hmacKey == nil || len(tca.hmacKey) == 0 {
		t.Fatal("Could not read hmacKey from TCA")
	}

	if tca.rootPreKey == nil || len(tca.rootPreKey) == 0 {
		t.Fatal("Could not read rootPreKey from TCA")
	}

	if tca.preKeys == nil || len(tca.preKeys) == 0 {
		t.Fatal("Could not read preKeys from TCA")
	}

}

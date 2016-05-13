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

var ca *CA

const (
	name = "TestCA"
)

func TestNewCA(t *testing.T) {

	//init the crypto layer
	if err := crypto.Init(); err != nil {
		t.Errorf("Failed initializing the crypto layer [%s]%", err)
	}

	//initialize logging to avoid panics in the current code
	LogInit(os.Stdout, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	//Create new CA
	ca := NewCA(name)
	if ca == nil {
		t.Error("could not create new CA")
	}

	//cleanup
	err := cleanupFiles(ca.path)
	if err != nil {
		t.Logf("Failed removing [%s] [%s]\n", ca.path, err)
	}

}

//check for file which we were expecting to create on the file system
func checkForFiles(path string, files []string) error {

	return nil
}

//cleanup files between and after tests
func cleanupFiles(path string) error {
	return os.RemoveAll(path)
}

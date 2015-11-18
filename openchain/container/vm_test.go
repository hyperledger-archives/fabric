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

package container

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	pb "github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
)

func TestMain(m *testing.M) {
	SetupTestConfig()
	os.Exit(m.Run())
}

func TestVM_ListImages(t *testing.T) {
	t.Skip("No need to invoke list images.")
	vm, err := NewVM()
	if err != nil {
		t.Fail()
		t.Logf("Error getting VM: %s", err)
	}
	err = vm.ListImages(context.TODO())
	if err != nil {
		t.Fail()
		t.Logf("Error listing images: %s", err)
	}
}

func TestVM_BuildImage_WritingGopathSource(t *testing.T) {
	t.Skip("This can be re-enabled if testing GOPATH writing to tar image.")
	vm, err := NewVM()
	if err != nil {
		t.Fail()
		t.Logf("Error getting VM: %s", err)
		return
	}
	inputbuf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(inputbuf)

	err = vm.writeGopathSrc(tw)
	ioutil.WriteFile("/tmp/chaincode_deployment.tar", inputbuf.Bytes(), 0644)

}

func TestVM_BuildImage_Peer(t *testing.T) {
	vm, err := NewVM()
	if err != nil {
		t.Fail()
		t.Logf("Error getting VM: %s", err)
		return
	}
	if err := vm.BuildPeerContainer(); err != nil {
		t.Fail()
		t.Log(err)
	}
}

func TestVM_BuildImage_Chaincode(t *testing.T) {
	vm, err := NewVM()
	if err != nil {
		t.Fail()
		t.Logf("Error getting VM: %s", err)
		return
	}
	// Build the spec
	chaincodePath := "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01"
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_GOLANG, ChaincodeID: &pb.ChaincodeID{Url: chaincodePath, Version: "0.1.0"}}
	if _, err := vm.BuildChaincodeContainer(spec); err != nil {
		t.Fail()
		t.Log(err)
	}
}

func TestVM_Chaincode_Compile(t *testing.T) {
	// vm, err := NewVM()
	// if err != nil {
	// 	t.Fail()
	// 	t.Logf("Error getting VM: %s", err)
	// 	return
	// }

	// if err := vm.BuildPeerContainer(); err != nil {
	// 	t.Fail()
	// 	t.Log(err)
	// }
	t.Skip("NOT IMPLEMENTED")
}

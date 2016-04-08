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
	"fmt"
	"io"

	"github.com/hyperledger/fabric/core/chaincode/shim"

	////////////HACK-A-TON/////////
	//"chaincodes/inprocchaincodesample"

	"golang.org/x/net/context"
)

type inprocChaincode struct {
	chaincode shim.Chaincode
	running bool
	args []string
	env []string
}

var (
	typeRegistry map[string] *inprocChaincode
)

func init() {
	typeRegistry = make(map[string]*inprocChaincode)
	//////////HACK-A-TON///////////
	//typeRegistry["chaincodes/inprocchaincodesample"] = &inprocChaincode{chaincode: &inprocchaincodesample.InProcChaincode{}, running: false}
}

//inProcVM is a vm. It is identified by a executable name
type inProcVM struct {
	id string
}

//for docker inputbuf is tar reader ready for use by docker.Client
//the stream from end client to peer could directly be this tar stream
//talk to docker daemon using docker Client and build the image
func (vm *inProcVM) build(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool, reader io.Reader) error {
	ipc := typeRegistry[id]
	if ipc == nil {
		return fmt.Errorf("%s not registered", id)
	}
	if ipc.running {
		return fmt.Errorf("%s running", id)
	}
	ipc.args = args
	ipc.env = env

	return nil
}

func (vm *inProcVM) start(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool) error {
	ipc := typeRegistry[id]
	if ipc == nil {
		return fmt.Errorf("%s not registered", id)
	}
	if ipc.running {
		return fmt.Errorf("%s running", id)
	}
	
	//TODO err := shim.Start(ipc.chaincode)
	return nil
}

func (vm *inProcVM) stop(ctxt context.Context, id string, timeout uint, dontkill bool, dontremove bool) error {
	ipc := typeRegistry[id]
	if ipc == nil {
		return fmt.Errorf("%s not registered", id)
	}
	if !ipc.running {
		return fmt.Errorf("%s not running", id)
	}
	//TODO 
	return nil
}

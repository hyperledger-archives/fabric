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

package inproccontroller

import (
	"fmt"
	"io"
	"time"

	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/op/go-logging"
	pb "github.com/hyperledger/fabric/protos"

	//HAAAACK
	tr "github.com/hyperledger/fabric/core/system_chaincode/timer"

	"golang.org/x/net/context"
)

type inprocChaincode struct {
	id		string
	chaincode	shim.Chaincode
	running		bool
	args		[]string
	env		[]string
}

var (
	typeRegistry map[string] *inprocChaincode
)

func init() {
	typeRegistry = make(map[string]*inprocChaincode)
}

var inprocLogger = logging.MustGetLogger("inproccontroller")

//InprocVM is a vm. It is identified by a executable name
type InprocVM struct {
	id string
}

//for docker inputbuf is tar reader ready for use by docker.Client
//the stream from end client to peer could directly be this tar stream
//talk to docker daemon using docker Client and build the image
func (vm *InprocVM) Deploy(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool, reader io.Reader) error {
	ipc := typeRegistry[id]
	if ipc != nil {
		return fmt.Errorf(fmt.Sprintf("%s already registered",id))
	}
	//..........................................................HAAACK............
	typeRegistry[id] = &inprocChaincode{id: id, chaincode: &tr.SystemTimerChaincode{}, running: false, args: args, env: env}
	inprocLogger.Debug("registered : %s", id)

	return nil
}

func (vm *InprocVM) launchInProc(ctxt context.Context, ipc *inprocChaincode, ccSupport ccintf.CCSupport) error {
	peerRcvCCSend := make(chan *pb.ChaincodeMessage)
	ccRcvPeerSend := make(chan *pb.ChaincodeMessage)
	go func() {
		inprocLogger.Debug("start chaincode %s", ipc.id)
		fmt.Sprintf("start chaincode %s\n", ipc.id)
		shim.StartInProc(ipc.env, ipc.args, ipc.chaincode,ccRcvPeerSend, peerRcvCCSend)
	}()
	go func() {
		inprocStream := newInProcStream(peerRcvCCSend, ccRcvPeerSend)
		inprocLogger.Debug("start chaincode-support for  %s", ipc.id)
		fmt.Sprintf("start chaincode-support for  %s\n", ipc.id)
		ccSupport.HandleChaincodeStream(ctxt, inprocStream)
		inprocLogger.Debug("aft start chaincode-support for  %s", ipc.id)
	}()

	return nil
}

func (vm *InprocVM) Start(ctxt context.Context, id string, args []string, env []string, attachstdin bool, attachstdout bool) error {
	ipc := typeRegistry[id]
	if ipc == nil {
		return fmt.Errorf(fmt.Sprintf("%s not registered",id))
	}
	if ipc.running {
		return fmt.Errorf(fmt.Sprintf("Removed container %s", id))
	}
	//TODO VALIDITY CHECKS ?
	ipc.id = id
	
	inprocLogger.Debug("extracting handler for : %s", id)

        ccSupport, ok := ctxt.Value(ccintf.GetCCHandlerKey()).(ccintf.CCSupport)
	if !ok || ccSupport == nil {
		return fmt.Errorf("in-process communication generator not supplied")
	}
	inprocLogger.Debug("extracted handler for : %s", id)
	fmt.Sprintf("extracted handler for : %s\n", id)
	vm.launchInProc(ctxt, ipc, ccSupport)

	time.Sleep(2*time.Second)
	return nil
}

func (vm *InprocVM) Stop(ctxt context.Context, id string, timeout uint, dontkill bool, dontremove bool) error {
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

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

	"golang.org/x/net/context"
)

type inprocContainer struct {
	chaincode	shim.Chaincode
	running		bool
	args		[]string
	env		[]string
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
	if ipc == nil {
		return fmt.Errorf(fmt.Sprintf("%s not registered. Please register the system chaincode in inprocinstances.go",id))
	}

	if ipc.chaincode == nil {
		return fmt.Errorf(fmt.Sprintf("%s system chaincode does not contain chaincode instance",id))
	}

	ipc.args = args
	ipc.env = env

	//FUTURE ... here is where we might check code for safety
	inprocLogger.Debug("registered : %s", id)

	return nil
}

func (ipc *inprocContainer) launchInProc(ctxt context.Context, id string, args []string, env []string, ccSupport ccintf.CCSupport) error {
	peerRcvCCSend := make(chan *pb.ChaincodeMessage)
	ccRcvPeerSend := make(chan *pb.ChaincodeMessage)
	var err error
	go func() {
		inprocLogger.Debug("start chaincode %s", id)
		fmt.Sprintf("start chaincode %s\n", id)
		if args == nil {
			args = ipc.args
		}
		if env == nil {
			env = ipc.env
		}
		err := shim.StartInProc(env, args, ipc.chaincode,ccRcvPeerSend, peerRcvCCSend)
		if err != nil {
			err = fmt.Errorf("start chaincode err %s", err)
			inprocLogger.Error(fmt.Sprintf("%s", err))
		}
	}()
	go func() {
		inprocStream := newInProcStream(peerRcvCCSend, ccRcvPeerSend)
		inprocLogger.Debug("start chaincode-support for  %s", id)
		fmt.Sprintf("start chaincode-support for  %s\n", id)
		ccSupport.HandleChaincodeStream(ctxt, inprocStream)
		inprocLogger.Debug("aft start chaincode-support for  %s", id)
	}()

	return err
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
	
        ccSupport, ok := ctxt.Value(ccintf.GetCCHandlerKey()).(ccintf.CCSupport)
	if !ok || ccSupport == nil {
		return fmt.Errorf("in-process communication generator not supplied")
	}

	 ipc.launchInProc(ctxt, id, args, env, ccSupport)

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

//GetVMFromName ignores the peer and network name as it just needs to be unique in process
func (vm *InprocVM) GetVMName(ccid ccintf.CCID) (string, error) {
	return ccid.ID,nil
}

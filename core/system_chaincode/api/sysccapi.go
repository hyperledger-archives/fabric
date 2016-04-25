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

package api

import (
	"encoding/base64"
	"fmt"
	"github.com/op/go-logging"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"golang.org/x/net/context"
	inproc "github.com/hyperledger/fabric/core/container/inproccontroller"
	pb "github.com/hyperledger/fabric/protos"
)

const (
	UBER = "uber"
)

var sysccLogger = logging.MustGetLogger("sysccapi")

var uberUp bool

// RegisterSysCC registers a system chaincode with the given path 
func RegisterSysCC(path string, o interface{}) error {
	syscc := o.(shim.Chaincode)
	if syscc == nil {
		sysccLogger.Warning(fmt.Sprintf("invalid chaincode %v", o))
		return fmt.Errorf(fmt.Sprintf("invalid chaincode %v", o))
	}
	err := inproc.Register(path, syscc)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("could not register (%s,%v): %s", path, syscc, err))
	}
	sysccLogger.Debug("system chaincode %s registered", path)
	return err
}

//SetUberUp sets uber is up or not
func SetUberUp(val bool) {
	uberUp = val
}

//UseUber should user be used
func UseUber() bool {
	//we really need a "policy" here... but for now use uber if its up
	return uberUp
}

//GetTransaction returns a transaction given args
//First implementation unmarshals base64 encoded string
func GetTransaction(args []string) (*pb.Transaction, []byte, error) {
	/***** NEW Approach construct image from ChaincodeID, func and args
	var depFunc string
	var depArgs []string
	if len(args) > 0 {
		depFunc = args[0]
		depArgs = args[1:]
	}
	fmt.Printf("Uber deploying %s, %v\n", depFunc, depArgs)
	**************/

	if len(args) != 1  {
		return nil, nil, fmt.Errorf("Invalid number of arguments %d", len(args))
	}

	//for now use old approach
	data, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		return nil, nil, fmt.Errorf("Error decoding chaincode deployment spec %s", err)
	}

	
	t := &pb.Transaction{}

	//TODO: need to use crypto decrypt t when using security
	err = proto.Unmarshal(data, t)
	if err != nil {
		err = fmt.Errorf("Transaction unmarshal failed: %s", err)
		sysccLogger.Error(fmt.Sprintf("%s", err))
		return nil, nil, err
	}

	return t, data, nil
}

func Deploy(t *pb.Transaction) error {
	_, err := chaincode.ExecuteWithoutCommit(context.Background(), chaincode.GetChain(chaincode.DefaultChain), t)
	return err
}

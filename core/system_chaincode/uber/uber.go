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

package uber

import (
	"fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

const (
	DEPLOY  = "deploy"
	START   = "start"
	STOP    = "stop"
	UPGRADE = "upgrade"
)

//errors
type InvalidFunctionErr string

func (f InvalidFunctionErr) Error() string {
    return fmt.Sprintf("invalid function to uber %s", string(f))
}

type InvalidArgsErr int

func (i InvalidArgsErr) Error() string {
    return fmt.Sprintf("invalid number of argument to uber %d", int(i))
}

// UberSysCC implements chaincode lifecycle and policies aroud it
type UberSysCC struct {
}

func (t *UberSysCC) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil, nil
}

// Invoke meta transaction to uber with functions "deploy", "start", "stop", "upgrade"
func (t *UberSysCC) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if function != DEPLOY && function != START && function != STOP && function != UPGRADE {
		return nil, InvalidFunctionErr(function)
	}

	if len(args) != 2 {
		return nil, InvalidArgsErr(len(args))
	}

	return nil, nil
}

// Query callback representing the query of a chaincode
func (t *UberSysCC) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil, nil
}

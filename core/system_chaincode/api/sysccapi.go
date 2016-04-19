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
	"fmt"
	"github.com/op/go-logging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	inproc "github.com/hyperledger/fabric/core/container/inproccontroller"
)

var sysccLogger = logging.MustGetLogger("sysccapi")

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

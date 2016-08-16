/*
 Copyright Digital Asset Holdings, LLC 2016 All Rights Reserved.

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

package noop

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	ld "github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/protos"
)

var logger = shim.NewLogger("noop")

type ledgerHandler interface {
	GetTransactionByUUID(txUUID string) (*protos.Transaction, error)
}

// SystemChaincode is type representing the chaincode
// In general, one should not use vars in memory that can hold state
// across invokes but this is used JUST for MOCKING
type SystemChaincode struct {
	mockLedgerH ledgerHandler
}

func (t *SystemChaincode) getLedger() ledgerHandler {
	if t.mockLedgerH == nil {
		lh, err := ld.GetLedger()
		if err == nil {
			return lh
		}
		panic("Chaincode is unable to get the ledger.")
	} else {
		return t.mockLedgerH
	}
}

// Init initailizes the system chaincode
func (t *SystemChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	logger.SetLevel(shim.LogDebug)
	logger.Debugf("NOOP INIT")
	return nil, nil
}

// Invoke runs an invocation on the system chaincode
func (t *SystemChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	switch function {
	case "execute":

		if len(args) < 1 {
			return nil, errors.New("execute operation must include single argument, the base64 encoded form of a byte sequence")
		}
		logger.Infof("Executing NOOP INVOKE")
		return nil, nil

	default:
		return nil, errors.New("Unsupported operation")
	}
}

// Query callback representing the query of a chaincode
func (t *SystemChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	switch function {
	case "getTran":
		if len(args) < 1 {
			return nil, errors.New("getTran operation must include a single argument, the TX hash hex")
		}
		logger.Infof("Executing NOOP QUERY")
		logger.Infof("--> %x", args[0])

		var txHashHex = args[0]
		var tx, txerr = t.getLedger().GetTransactionByUUID(txHashHex)
		if nil != txerr || nil == tx {
			return nil, txerr
		}
		newCCIS := &protos.ChaincodeInvocationSpec{}
		var merr = proto.Unmarshal(tx.Payload, newCCIS)
		if nil != merr {
			return nil, merr
		}
		var data = newCCIS.ChaincodeSpec.CtorMsg.Args[0]
		var dataInByteForm, b64err = base64.StdEncoding.DecodeString(data)
		if b64err != nil {
			return nil, fmt.Errorf("Error in decoding from Base64:  %s", b64err)
		}
		return dataInByteForm, nil
	default:
		return nil, errors.New("Unsupported operation")
	}
}

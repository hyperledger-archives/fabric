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

package chaincode

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/ledger"
	pb "github.com/openblockchain/obc-peer/protos"
)

//Execute - execute transaction or a query
func  Execute(ctxt context.Context, chain *ChainletSupport, t *pb.Transaction) ([]byte, error) {
	var err error
	if t.Type == pb.Transaction_CHAINLET_NEW {
		_,err := chain.DeployChaincode(ctxt, t)
		if err != nil {
			return nil, fmt.Errorf("Failed to deploy chaincode spec(%s)", err)
		}

		//launch and wait for ready
		_,_,err = chain.LaunchChaincode(ctxt, t)
		if err != nil {
			//TODO rollback transaction as init might have set state
			return nil, fmt.Errorf("Failed to launch chaincode spec(%s)", err)
		}
	} else if t.Type == pb.Transaction_CHAINLET_EXECUTE || t.Type == pb.Transaction_CHAINLET_QUERY {
		//will launch if necessary (and wait for ready)
		cID,cMsg,err := chain.LaunchChaincode(ctxt, t)
		if err != nil {
			return nil, fmt.Errorf("Failed to launch chaincode spec(%s)", err)
		}

		//this should work because it worked above...
		chaincode, _ := getChaincodeID(cID)

		if(err != nil){
			return nil, fmt.Errorf("Failed to stablish stream to container %s", chaincode)
		}
	
		// TODO: Need to comment next line and uncomment call to getTimeout, when transaction blocks are being created	
		timeout := time.Duration(30000) * time.Millisecond
		//timeout, err := getTimeout(cID)
		
		if(err != nil){
			return nil, fmt.Errorf("Failed to retrieve chaincode spec(%s)", err)
		}
		
		var ccMsg *pb.ChaincodeMessage
		if t.Type == pb.Transaction_CHAINLET_EXECUTE {
			ccMsg,err = createTransactionMessage(t.Uuid, cMsg)
			if err != nil {
				return nil, fmt.Errorf("Failed to transaction message(%s)", err)
			}
		} else {
			ccMsg,err = createQueryMessage(t.Uuid, cMsg)
			if err != nil {
				return nil, fmt.Errorf("Failed to query message(%s)", err)
			}
		}

		resp,err := chain.Execute(ctxt, chaincode, ccMsg, timeout)
		if err != nil {
			//TODO rollback transaction....
			return nil, fmt.Errorf("Failed to execute transaction(%s)", err)
		} else if resp == nil {
			//TODO rollback transaction....
			return nil, fmt.Errorf("Failed to receive a response for (%s)", t.Uuid)
		} else {
			if resp.Type == pb.ChaincodeMessage_COMPLETED {
				return 	resp.Payload, nil
			}
			return resp.Payload, fmt.Errorf("receive a response for (%s) but in invalid state(%d)", t.Uuid, resp.Type)
		}

	} else {
		err = fmt.Errorf("Invalid transaction type %s", t.Type.String())
	}
	return  nil,err
}

//ExecuteTransactions - will execute transactions on the array one by one
//will return an array of errors one for each transaction. If the execution
//succeeded, array element will be nil. returns state hash
func  ExecuteTransactions(ctxt context.Context, cname ChainName, xacts []*pb.Transaction) ([]byte, []error) {
	var chain = GetChain(cname)
	if chain == nil {
		panic(fmt.Sprintf("[ExecuteTransactions]Chain %s not found\n", cname))
	}
	errs := make([]error, len(xacts)+1)
	for i, t := range xacts {
		_,errs[i] = Execute(ctxt,chain,t)
	}
	ledger, hasherr := ledger.GetLedger()
	var statehash []byte
	if hasherr == nil {
		statehash, hasherr = ledger.GetTempStateHash()
	}
	errs[len(errs)-1] = hasherr
	return statehash, errs
}

var errFailedToGetChainCodeSpecForTransaction = errors.New("Failed to get ChainCodeSpec from Transaction")

func getTimeout(cID *pb.ChainletID) (time.Duration, error) {
	ledger, err := ledger.GetLedger()
	if err == nil {
		chaincodeID := cID.Url + ":" + cID.Version
		txUUID, err := ledger.GetState(chaincodeID, "github.com_openblockchain_obc-peer_chaincode_id", true)
		if err == nil {
			tx, err := ledger.GetTransactionByUUID(string(txUUID))
			if err == nil {
				chainletDeploymentSpec := &pb.ChainletDeploymentSpec{}
				proto.Unmarshal(tx.Payload, chainletDeploymentSpec)
				chainletSpec := chainletDeploymentSpec.GetChainletSpec()
				timeout := time.Duration(time.Duration(chainletSpec.Timeout) * time.Millisecond)
				return timeout, nil
			}
		}
	}
	
	return -1, errFailedToGetChainCodeSpecForTransaction
}

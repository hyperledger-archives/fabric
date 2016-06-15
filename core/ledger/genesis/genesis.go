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

package genesis

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/protos"
	"github.com/op/go-logging"
)

var genesisLogger = logging.MustGetLogger("genesis")

var makeGenesisError error
var once sync.Once

// MakeGenesis creates the genesis block based on configuration in core.yaml
// and adds it to the blockchain.
func MakeGenesis() error {
	once.Do(func() {
		ledger, err := ledger.GetLedger()
		if err != nil {
			makeGenesisError = err
			return
		}

		var genesisBlockExists bool
		if ledger.GetBlockchainSize() == 0 {
			genesisLogger.Info("Creating genesis block.")
			ledger.BeginTxBatch(0)
		} else {
			genesisBlockExists = true
		}

		var genesisTransactions []*protos.Transaction

		defer func() {
			if !genesisBlockExists && makeGenesisError == nil {
				genesisLogger.Infof("Adding %d system chaincodes to the genesis block.", len(genesisTransactions))
				ledger.CommitTxBatch(0, genesisTransactions, nil, nil)
			}
		}()

		//We are disabling the validity period deployment for now, we shouldn't even allow it if it's enabled in the configuration
		allowDeployValidityPeriod := false

		if isDeploySystemChaincodeEnabled() && allowDeployValidityPeriod {
			vpTransaction, deployErr := deployUpdateValidityPeriodChaincode(genesisBlockExists)

			if deployErr != nil {
				genesisLogger.Errorf("Error deploying validity period system chaincode for genesis block: %s", deployErr)
				makeGenesisError = deployErr
				return
			}

			genesisTransactions = append(genesisTransactions, vpTransaction)
		}

		if getGenesis() == nil {
			genesisLogger.Info("No genesis block chaincodes defined.")
		} else {

			chaincodes, chaincodesOK := genesis["chaincodes"].(map[interface{}]interface{})
			if !chaincodesOK {
				genesisLogger.Info("No genesis block chaincodes defined.")
				ledger.CommitTxBatch(0, genesisTransactions, nil, nil)
				return
			}

			genesisLogger.Debugf("Genesis chaincodes are %s", chaincodes)

			for i := range chaincodes {
				name := i.(string)
				genesisLogger.Debugf("Chaincode %s", name)

				chaincode := chaincodes[name]
				chaincodeMap, chaincodeMapOK := chaincode.(map[interface{}]interface{})
				if !chaincodeMapOK {
					genesisLogger.Errorf("Invalid chaincode defined in genesis configuration: %s", chaincode)
					makeGenesisError = fmt.Errorf("Invalid chaincode defined in genesis configuration: %s", chaincode)
					return
				}

				path, pathOK := chaincodeMap["path"].(string)
				if !pathOK {
					genesisLogger.Errorf("Invalid chaincode URL defined in genesis configuration: %s", chaincodeMap["path"])
					makeGenesisError = fmt.Errorf("Invalid chaincode URL defined in genesis configuration: %s", chaincodeMap["path"])
					return
				}

				chaincodeType, chaincodeTypeOK := chaincodeMap["type"].(string)
				if !chaincodeTypeOK {
					genesisLogger.Errorf("Invalid chaincode type defined in genesis configuration: %s", chaincodeMap["type"])
					makeGenesisError = fmt.Errorf("Invalid chaincode type defined in genesis configuration: %s", chaincodeMap["type"])
					return
				}

				if chaincodeType == "" {
					chaincodeType = "GOLANG"
				}

				chaincodeID := &protos.ChaincodeID{Path: path, Name: name}

				genesisLogger.Debugf("Genesis chaincodeID %s", chaincodeID)

				constructorMap, constructorMapOK := chaincodeMap["constructor"].(map[interface{}]interface{})
				if !constructorMapOK {
					genesisLogger.Errorf("Invalid chaincode constructor defined in genesis configuration: %s", chaincodeMap["constructor"])
					makeGenesisError = fmt.Errorf("Invalid chaincode constructor defined in genesis configuration: %s", chaincodeMap["constructor"])
					return
				}

				var spec protos.ChaincodeSpec
				if constructorMap == nil {
					genesisLogger.Debug("Genesis chaincode has no constructor.")
					spec = protos.ChaincodeSpec{Type: protos.ChaincodeSpec_Type(protos.ChaincodeSpec_Type_value[chaincodeType]), ChaincodeID: chaincodeID}
				} else {

					_, ctorArgsOK := constructorMap["args"]
					if !ctorArgsOK {
						genesisLogger.Errorf("Invalid chaincode constructor args defined in genesis configuration: %s", constructorMap["args"])
						makeGenesisError = fmt.Errorf("Invalid chaincode constructor args defined in genesis configuration: %s", constructorMap["args"])
						return
					}

					ctorArgs, ctorArgsOK := constructorMap["args"].([]interface{})
					var ctorArgsStringArray []string
					if ctorArgsOK {
						genesisLogger.Debugf("Genesis chaincode constructor args %s", ctorArgs)
						for j := 0; j < len(ctorArgs); j++ {
							ctorArgsStringArray = append(ctorArgsStringArray, ctorArgs[j].(string))
						}
					}
					spec = protos.ChaincodeSpec{Type: protos.ChaincodeSpec_Type(protos.ChaincodeSpec_Type_value[chaincodeType]), ChaincodeID: chaincodeID, CtorMsg: &protos.ChaincodeInput{Args: ctorArgsStringArray}}
				}

				transaction, _, deployErr := DeployLocal(context.Background(), &spec, genesisBlockExists)
				if deployErr != nil {
					genesisLogger.Errorf("Error deploying chaincode for genesis block: %s", deployErr)
					makeGenesisError = deployErr
					return
				}

				genesisTransactions = append(genesisTransactions, transaction)

			} //for

		} //else
	})
	return makeGenesisError
}

//BuildLocal builds a given chaincode code
func BuildLocal(context context.Context, spec *protos.ChaincodeSpec) (*protos.ChaincodeDeploymentSpec, error) {
	genesisLogger.Debugf("Received build request for chaincode spec: %v", spec)
	var codePackageBytes []byte
	/*****  We will need this only when we support non-go SYSTEM chaincode ****
	if getMode() != chaincode.DevModeUserRunsChaincode {
		if err := core.CheckSpec(spec); err != nil {
			genesisLogger.Debugf("check spec failed: %s", err)
			return nil, err
		}
		// Build the spec
		var err error
		codePackageBytes, err = container.GetChaincodePackageBytes(spec)
		if err != nil {
			genesisLogger.Errorf("Error getting VM: %s", err)
			return nil, err
		}
	}
	*********/
	chaincodeDeploymentSpec := &protos.ChaincodeDeploymentSpec{ExecEnv: protos.ChaincodeDeploymentSpec_SYSTEM, ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// DeployLocal deploys the supplied chaincode image to the local peer
func DeployLocal(ctx context.Context, spec *protos.ChaincodeSpec, gbexists bool) (*protos.Transaction, []byte, error) {
	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := BuildLocal(ctx, spec)

	if err != nil {
		genesisLogger.Errorf("Error deploying chaincode spec: %v\n\n error: %s", spec, err)
		return nil, nil, err
	}

	var transaction *protos.Transaction
	if gbexists {
		ledger, err := ledger.GetLedger()
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to get handle to ledger (%s)", err)
		}
		transaction, err = ledger.GetTransactionByUUID(chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name)
		if err != nil {
			genesisLogger.Warningf("cannot get deployment transaction for %s - %s", chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name, err)
			transaction = nil
		} else {
			genesisLogger.Debugf("deployment transaction for %s exists", chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name)
		}
	}

	if transaction == nil {
		transaction, err = protos.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("Error deploying chaincode: %s ", err)
		}
	}

	//chaincode.NewChaincodeSupport(chaincode.DefaultChain, peer.GetPeerEndpoint, false, 120000)
	// The secHelper is set during creat ChaincodeSupport, so we don't need this step
	//ctx = context.WithValue(ctx, "security", secCxt)
	result, _, err := chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
	return transaction, result, err
}

func deployUpdateValidityPeriodChaincode(gbexists bool) (*protos.Transaction, error) {
	//TODO It should be configurable, not hardcoded
	vpChaincodePath := "github.com/hyperledger/fabric/core/system_chaincode/validity_period_update"
	vpFunction := "init"

	//TODO: this should be the login token for the component in charge of the validity period update.
	//This component needs to be registered in the system to be able to invoke the update validity period system chaincode.
	vpToken := "system_chaincode_invoker"

	var vpCtorArgsStringArray []string

	validityPeriodSpec := &protos.ChaincodeSpec{Type: protos.ChaincodeSpec_GOLANG,
		ChaincodeID: &protos.ChaincodeID{Path: vpChaincodePath,
			Name: "",
		},
		CtorMsg: &protos.ChaincodeInput{Function: vpFunction,
			Args: vpCtorArgsStringArray,
		},
	}

	validityPeriodSpec.SecureContext = string(vpToken)

	vpTransaction, _, deployErr := DeployLocal(context.Background(), validityPeriodSpec, gbexists)

	if deployErr != nil {
		genesisLogger.Errorf("Error deploying validity period chaincode for genesis block: %s", deployErr)
		makeGenesisError = deployErr
		return nil, deployErr
	}

	return vpTransaction, nil
}

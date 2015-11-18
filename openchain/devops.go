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

package openchain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/peer"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

var devopsLogger = logging.MustGetLogger("devops")

// NewDevopsServer creates and returns a new Devops server instance.
func NewDevopsServer() *Devops {
	d := new(Devops)
	return d
}

// Devops implementation of Devops services
type Devops struct {
}

// Build builds the supplied chaincode image
func (*Devops) Build(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	mode := viper.GetString("chaincode.chaincoderunmode")
	var codePackageBytes []byte
	if mode != chaincode.DevModeUserRunsChaincode {
		devopsLogger.Debug("Received build request for chaincode spec: %v", spec)
		if err := checkSpec(spec); err != nil {
			return nil, err
		}
		// Get new VM and as for building of container image
		vm, err := container.NewVM()
		if err != nil {
			devopsLogger.Error(fmt.Sprintf("Error getting VM: %s", err))
			return nil, err
		}
		// Build the spec
		codePackageBytes, err = vm.BuildChaincodeContainer(spec)
		if err != nil {
			devopsLogger.Error(fmt.Sprintf("Error getting VM: %s", err))
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// Deploy deploys the supplied chaincode image to the validators through a transaction
func (d *Devops) Deploy(ctx context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := d.Build(ctx, spec)

	if err != nil {
		devopsLogger.Error(fmt.Sprintf("Error deploying chaincode spec: %v\n\n error: %s", spec, err))
		return nil, err
	}
	//devopsLogger.Debug("returning status: %s", status)
	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		devopsLogger.Error(fmt.Sprintf("Error generating UUID: %s", uuidErr))
		return nil, uuidErr
	}
	transaction, err := pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	mode := viper.GetString("chaincode.chaincoderunmode")

	if mode == chaincode.DevModeUserRunsChaincode {
		_, execErr := chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
		return chaincodeDeploymentSpec, execErr
	}

	//we are not in "dev mode", we have to pass requests to remote validator via stream
	peerAddress, err := GetRootNode()
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	// Construct the transactions block.
	transactionBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{transaction}}
	return chaincodeDeploymentSpec, peer.SendTransactionsToPeer(peerAddress, transactionBlock)
}

func (d *Devops) invokeOrQuery(ctx context.Context, chaincodeInvocationSpec *pb.ChaincodeInvocationSpec, invoke bool) (*pb.DevopsResponse, error) {

	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		devopsLogger.Error(fmt.Sprintf("Error generating UUID: %s", uuidErr))
		return nil, uuidErr
	}
	var transaction *pb.Transaction
	var err error
	if invoke {
		transaction, err = pb.NewChaincodeExecute(chaincodeInvocationSpec, uuid, pb.Transaction_CHAINLET_EXECUTE)
	} else {
		transaction, err = pb.NewChaincodeExecute(chaincodeInvocationSpec, uuid, pb.Transaction_CHAINLET_QUERY)
	}
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}

	mode := viper.GetString("chaincode.chaincoderunmode")

	//in dev mode, we invoke locally (whether user runs chaincode or validator does)
	if mode == chaincode.DevModeUserRunsChaincode {
		if invoke {
			chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
			return &pb.DevopsResponse{Status: pb.DevopsResponse_SUCCESS, Msg: []byte(transaction.Uuid)}, nil
		}
		payload, execErr := chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
		if execErr != nil {
			return &pb.DevopsResponse{Status: pb.DevopsResponse_FAILURE}, execErr
		}
		return &pb.DevopsResponse{Status: pb.DevopsResponse_SUCCESS, Msg: payload}, nil
	}

	/** TODO
		  * - hook up with latest code so we can submit the transaction/query to remote validator
		  * - in particular, we need synch response for query
			peerAddress, err := GetRootNode()
			if err != nil {
				return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
			}
			// Construct the transactions block.
			transactionBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{transaction}}
			devopsLogger.Debug("Sending invocation transaction (%s) to validator at address %s", transactionBlock.Transactions, peerAddress)
			//return &google_protobuf.Empty{}, SendTransactionsToPeer(peerAddress, transactionBlock)
			return &google_protobuf.Empty{}, nil
	        ***/
	return nil, nil
}

// Invoke performs the supplied invocation on the specified chaincode through a transaction
func (d *Devops) Invoke(ctx context.Context, chaincodeInvocationSpec *pb.ChaincodeInvocationSpec) (*pb.DevopsResponse, error) {
	return d.invokeOrQuery(ctx, chaincodeInvocationSpec, true)
}

// Query performs the supplied query on the specified chaincode through a transaction
func (d *Devops) Query(ctx context.Context, chaincodeInvocationSpec *pb.ChaincodeInvocationSpec) (*pb.DevopsResponse, error) {
	return d.invokeOrQuery(ctx, chaincodeInvocationSpec, false)
}

// Checks to see if chaincode resides within current package capture for language.
func checkSpec(spec *pb.ChaincodeSpec) error {
	// Don't allow nil value
	if spec == nil {
		return errors.New("Expected chaincode specification, nil received")
	}

	// Only allow GOLANG type at the moment
	if spec.Type != pb.ChaincodeSpec_GOLANG {
		return fmt.Errorf("Only support '%s' currently", pb.ChaincodeSpec_GOLANG)
	}
	if err := checkGolangSpec(spec); err != nil {
		return err
	}
	devopsLogger.Debug("Validated spec:  %v", spec)

	// Check the version
	_, err := semver.Make(spec.ChaincodeID.Version)
	return err
}

func checkGolangSpec(spec *pb.ChaincodeSpec) error {
	pathToCheck := filepath.Join(os.Getenv("GOPATH"), "src", spec.ChaincodeID.Url)
	exists, err := pathExists(pathToCheck)
	if err != nil {
		return fmt.Errorf("Error validating chaincode path: %s", err)
	}
	if !exists {
		return fmt.Errorf("Path to chaincode does not exist: %s", spec.ChaincodeID.Url)
	}
	return nil
}

// Returns whether the given file or directory exists or not
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

//BuildLocal builds a given chaincode code
func BuildLocal(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	devopsLogger.Debug("Received build request for chaincode spec: %v", spec)
	mode := viper.GetString("chaincode.chaincoderunmode")
	var codePackageBytes []byte
	if mode != chaincode.DevModeUserRunsChaincode {
		if err := checkSpec(spec); err != nil {
			devopsLogger.Debug("check spec failed: %s", err)
			return nil, err
		}
		// Get new VM and as for building of container image
		vm, err := container.NewVM()
		if err != nil {
			devopsLogger.Error(fmt.Sprintf("Error getting VM: %s", err))
			return nil, err
		}
		// Build the spec
		codePackageBytes, err = vm.BuildChaincodeContainer(spec)
		if err != nil {
			devopsLogger.Error(fmt.Sprintf("Error getting VM: %s", err))
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// DeployLocal deploys the supplied chaincode image to the local peer
func DeployLocal(ctx context.Context, spec *pb.ChaincodeSpec) ([]byte, error) {
	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := BuildLocal(ctx, spec)

	if err != nil {
		devopsLogger.Error(fmt.Sprintf("Error deploying chaincode spec: %v\n\n error: %s", spec, err))
		return nil, err
	}
	//devopsLogger.Debug("returning status: %s", status)
	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		devopsLogger.Error(fmt.Sprintf("Error generating UUID: %s", uuidErr))
		return nil, uuidErr
	}
	transaction, err := pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	return chaincode.Execute(ctx, chaincode.GetChain(chaincode.DefaultChain), transaction)
}

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
	google_protobuf "google/protobuf"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
	"golang.org/x/net/context"
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
func (*Devops) Build(context context.Context, spec *pb.ChainletSpec) (*pb.ChainletDeploymentSpec, error) {
	devopsLogger.Debug("Received build request for chainlet spec: %v", spec)
	if err := checkSpec(spec); err != nil {
		return nil, err
	}
	// Get new VM and as for building of container image
	vm, err := NewVM()
	if err != nil {
		devopsLogger.Error(fmt.Sprintf("Error getting VM: %s", err))
		return nil, err
	}
	// Build the spec
	codePackageBytes, err := vm.BuildChaincodeContainer(spec)
	if err != nil {
		devopsLogger.Error(fmt.Sprintf("Error getting VM: %s", err))
		return nil, err
	}
	chainletDeploymentSepc := &pb.ChainletDeploymentSpec{ChainletSpec: spec, CodePackage: codePackageBytes}
	return chainletDeploymentSepc, nil
}

// Deploy deploys the supplied chaincode image to the validators through a transaction
func (d *Devops) Deploy(ctx context.Context, spec *pb.ChainletSpec) (*pb.ChainletDeploymentSpec, error) {
	// First build and get the deployment spec
	chainletDeploymentSepc, err := d.Build(ctx, spec)

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
	transaction, err := pb.NewChainletDeployTransaction(chainletDeploymentSepc, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	peerAddress, err := GetRootNode()
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	// Construct the transactions block.
	transactionBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{transaction}}
	return chainletDeploymentSepc, SendTransactionsToPeer(peerAddress, transactionBlock)
}

// Invoke performs the supplied invocation on the specified chaincode through a transaction
func (d *Devops) Invoke(ctx context.Context, chaincodeInvocation *pb.ChaincodeInvocation) (*google_protobuf.Empty, error) {

	// Now create the Transactions message and send to Peer.
	uuid, uuidErr := util.GenerateUUID()
	if uuidErr != nil {
		devopsLogger.Error(fmt.Sprintf("Error generating UUID: %s", uuidErr))
		return nil, uuidErr
	}
	transaction, err := pb.NewChainletInvokeTransaction(chaincodeInvocation, uuid)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	peerAddress, err := GetRootNode()
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	// Construct the transactions block.
	transactionBlock := &pb.TransactionBlock{Transactions: []*pb.Transaction{transaction}}
	peerLogger.Debug("Sending invocation transaction (%s) to validator at address %s", transactionBlock.Transactions, peerAddress)
	//return &google_protobuf.Empty{}, SendTransactionsToPeer(peerAddress, transactionBlock)
	return &google_protobuf.Empty{}, nil
}

// Checks to see if chaincode resides within current package capture for language.
func checkSpec(spec *pb.ChainletSpec) error {
	// Don't allow nil value
	if spec == nil {
		return errors.New("Expected chaincode specification, nil received")
	}

	// Only allow GOLANG type at the moment
	if spec.Type != pb.ChainletSpec_GOLANG {
		return fmt.Errorf("Only support '%s' currently", pb.ChainletSpec_GOLANG)
	}
	if err := checkGolangSpec(spec); err != nil {
		return err
	}
	devopsLogger.Debug("Validated spec:  %v", spec)

	// Check the version
	_, err := semver.Make(spec.ChainletID.Version)
	return err
}

func checkGolangSpec(spec *pb.ChainletSpec) error {
	pathToCheck := filepath.Join(os.Getenv("GOPATH"), "src", spec.ChainletID.Url)
	exists, err := pathExists(pathToCheck)
	if err != nil {
		return fmt.Errorf("Error validating chaincode path: %s", err)
	}
	if !exists {
		return fmt.Errorf("Path to chaincode does not exist: %s", spec.ChainletID.Url)
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

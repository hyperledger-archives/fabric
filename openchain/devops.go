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
	"golang.org/x/net/context"

	pb "github.com/openblockchain/obc-peer/protos"
)

var devops_logger = logging.MustGetLogger("devops")

func NewDevopsServer() *devops {
	d := new(devops)
	return d
}

type devops struct {
}

func (*devops) Build(context context.Context, spec *pb.ChainletSpec) (*pb.ChainletDeploymentSpec, error) {
	devops_logger.Debug("Received build request for chainlet spec: %v", spec)
	if err := checkSpec(spec); err != nil {
		return nil, err
	}
	// Get new VM and as for building of container image
	vm, err := NewVM()
	if err != nil {
		devops_logger.Error("Error getting VM: %s", err)
		return nil, err
	}
	// Build the spec
	codePackageBytes, err := vm.BuildChaincodeContainer(spec)
	if err != nil {
		devops_logger.Error("Error getting VM: %s", err)
		return nil, err
	}
	chainletDeploymentSepc := &pb.ChainletDeploymentSpec{ChainletSpec: spec, CodePackage: codePackageBytes}
	return chainletDeploymentSepc, nil
}

func (d *devops) Deploy(ctx context.Context, spec *pb.ChainletSpec) (*pb.ChainletDeploymentSpec, error) {
	// First build and get the deployment spec
	chainletDeploymentSepc, err := d.Build(ctx, spec)

	if err != nil {
		devops_logger.Error("Error deploying chaincode spec: %v\n\n error: %s", spec, err)
		return nil, err
	}
	//devops_logger.Debug("returning status: %s", status)
	// Now create the Transactions message and send to Peer.
	transaction, err := pb.NewChainletDeployTransaction(chainletDeploymentSepc)
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	peerAddress, err := GetRootNode()
	if err != nil {
		return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
	}
	// Construct the Transactions Message
	transactionsMessage := &pb.TransactionsMessage{Transactions: []*pb.Transaction{transaction}}
	return chainletDeploymentSepc, SendTransactionsToPeer(peerAddress, transactionsMessage)
}

// Checks to see if chaincode resides within current package capture for language.
func checkSpec(spec *pb.ChainletSpec) error {
	// Don't allow nil value
	if spec == nil {
		return errors.New("Expected chaincode specification, nil received")
	}

	// Only allow GOLANG type at the moment
	if spec.Type != pb.ChainletSpec_GOLANG {
		return errors.New(fmt.Sprintf("Only support '%s' currently", pb.ChainletSpec_GOLANG))
	}
	if err := checkGolangSpec(spec); err != nil {
		return err
	}
	devops_logger.Debug("Validated spec:  %v", spec)

	// Check the version
	_, err := semver.Make(spec.ChainletID.Version)
	return err
}

func checkGolangSpec(spec *pb.ChainletSpec) error {
	pathToCheck := filepath.Join(os.Getenv("GOPATH"), "src", spec.ChainletID.Url)
	exists, err := pathExists(pathToCheck)
	if err != nil {
		return errors.New(fmt.Sprintf("Error validating chaincode path: %s", err))
	}
	if !exists {
		return errors.New(fmt.Sprintf("Path to chaincode does not exist: %s", spec.ChainletID.Url))
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

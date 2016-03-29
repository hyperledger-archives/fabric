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
	"net/url"
	"os"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/peer"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

var devopsLogger = logging.MustGetLogger("devops")

// NewDevopsServer creates and returns a new Devops server instance.
func NewDevopsServer(coord peer.MessageHandlerCoordinator) *Devops {
	d := new(Devops)
	d.coord = coord
	return d
}

// Devops implementation of Devops services
type Devops struct {
	coord peer.MessageHandlerCoordinator
}

// Login establishes the security context with the Devops service
func (d *Devops) Login(ctx context.Context, secret *pb.Secret) (*pb.Response, error) {
	if err := crypto.RegisterClient(secret.EnrollId, nil, secret.EnrollId, secret.EnrollSecret); nil != err {
		return &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(err.Error())}, nil
	}
	return &pb.Response{Status: pb.Response_SUCCESS}, nil

	// TODO: Handle timeout and expiration
}

// Build builds the supplied chaincode image
func (*Devops) Build(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	mode := viper.GetString("chaincode.mode")
	var codePackageBytes []byte
	if mode != chaincode.DevModeUserRunsChaincode {
		devopsLogger.Debug("Received build request for chaincode spec: %v", spec)
		if err := CheckSpec(spec); err != nil {
			return nil, err
		}

		vm, err := container.NewVM()
		if err != nil {
			return nil, fmt.Errorf("Error getting vm")
		}

		codePackageBytes, err = vm.BuildChaincodeContainer(spec)
		if err != nil {
			err = fmt.Errorf("Error getting chaincode package bytes: %s", err)
			devopsLogger.Error(fmt.Sprintf("%s", err))
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// get chaincode bytes
func (*Devops) getChaincodeBytes(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	mode := viper.GetString("chaincode.mode")
	var codePackageBytes []byte
	if mode != chaincode.DevModeUserRunsChaincode {
		devopsLogger.Debug("Received build request for chaincode spec: %v", spec)
		var err error
		if err = CheckSpec(spec); err != nil {
			return nil, err
		}

		codePackageBytes, err = container.GetChaincodePackageBytes(spec)
		if err != nil {
			err = fmt.Errorf("Error getting chaincode package bytes: %s", err)
			devopsLogger.Error(fmt.Sprintf("%s", err))
			return nil, err
		}
	}
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

// Deploy deploys the supplied chaincode image to the validators through a transaction
func (d *Devops) Deploy(ctx context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	// get the deployment spec
	chaincodeDeploymentSpec, err := d.getChaincodeBytes(ctx, spec)

	if err != nil {
		devopsLogger.Error(fmt.Sprintf("Error deploying chaincode spec: %v\n\n error: %s", spec, err))
		return nil, err
	}

	// Now create the Transactions message and send to Peer.

	transID := chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name

	var tx *pb.Transaction
	var sec crypto.Client

	if viper.GetBool("security.enabled") {
		if devopsLogger.IsEnabledFor(logging.DEBUG) {
			devopsLogger.Debug("Initializing secure devops using context %s", spec.SecureContext)
		}
		sec, err = crypto.InitClient(spec.SecureContext, nil)
		defer crypto.CloseClient(sec)

		// remove the security context since we are no longer need it down stream
		spec.SecureContext = ""

		if nil != err {
			return nil, err
		}

		if devopsLogger.IsEnabledFor(logging.DEBUG) {
			devopsLogger.Debug("Creating secure transaction %s", transID)
		}
		tx, err = sec.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, transID)
		if nil != err {
			return nil, err
		}
	} else {
		if devopsLogger.IsEnabledFor(logging.DEBUG) {
			devopsLogger.Debug("Creating deployment transaction (%s)", transID)
		}
		tx, err = pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, transID)
		if err != nil {
			return nil, fmt.Errorf("Error deploying chaincode: %s ", err)
		}
	}

	if devopsLogger.IsEnabledFor(logging.DEBUG) {
		devopsLogger.Debug("Sending deploy transaction (%s) to validator", tx.Uuid)
	}
	resp := d.coord.ExecuteTransaction(tx)
	if resp.Status == pb.Response_FAILURE {
		err = fmt.Errorf(string(resp.Msg))
	}

	return chaincodeDeploymentSpec, err
}

func (d *Devops) invokeOrQuery(ctx context.Context, chaincodeInvocationSpec *pb.ChaincodeInvocationSpec, invoke bool) (*pb.Response, error) {

	if chaincodeInvocationSpec.ChaincodeSpec.ChaincodeID.Name == "" {
		return nil, fmt.Errorf("name not given for invoke/query")
	}

	// Now create the Transactions message and send to Peer.
	uuid := util.GenerateUUID()
	var transaction *pb.Transaction
	var err error
	var sec crypto.Client
	if viper.GetBool("security.enabled") {
		if devopsLogger.IsEnabledFor(logging.DEBUG) {
			devopsLogger.Debug("Initializing secure devops using context %s", chaincodeInvocationSpec.ChaincodeSpec.SecureContext)
		}
		sec, err = crypto.InitClient(chaincodeInvocationSpec.ChaincodeSpec.SecureContext, nil)
		defer crypto.CloseClient(sec)
		// remove the security context since we are no longer need it down stream
		chaincodeInvocationSpec.ChaincodeSpec.SecureContext = ""
		if nil != err {
			return nil, err
		}
	}
	transaction, err = d.createExecTx(chaincodeInvocationSpec, uuid, invoke, sec)
	if err != nil {
		return nil, err
	}
	if devopsLogger.IsEnabledFor(logging.DEBUG) {
		devopsLogger.Debug("Sending invocation transaction (%s) to validator", transaction.Uuid)
	}
	resp := d.coord.ExecuteTransaction(transaction)
	if resp.Status == pb.Response_FAILURE {
		err = fmt.Errorf(string(resp.Msg))
	} else {
		if !invoke && nil != sec && viper.GetBool("security.privacy") {
			if resp.Msg, err = sec.DecryptQueryResult(transaction, resp.Msg); nil != err {
				devopsLogger.Debug("Failed decrypting query transaction result %s", string(resp.Msg[:]))
				//resp = &pb.Response{Status: pb.Response_FAILURE, Msg: []byte(err.Error())}
			}
		}
	}
	return resp, err
}

func (d *Devops) createExecTx(spec *pb.ChaincodeInvocationSpec, uuid string, invokeTx bool, sec crypto.Client) (*pb.Transaction, error) {
	var tx *pb.Transaction
	var err error
	if nil != sec {
		if devopsLogger.IsEnabledFor(logging.DEBUG) {
			devopsLogger.Debug("Creating secure invocation transaction %s", uuid)
		}
		if invokeTx {
			tx, err = sec.NewChaincodeExecute(spec, uuid)
		} else {
			tx, err = sec.NewChaincodeQuery(spec, uuid)
		}
		if nil != err {
			return nil, err
		}
	} else {
		if devopsLogger.IsEnabledFor(logging.DEBUG) {
			devopsLogger.Debug("Creating invocation transaction (%s)", uuid)
		}
		var t pb.Transaction_Type
		if invokeTx {
			t = pb.Transaction_CHAINCODE_EXECUTE
		} else {
			t = pb.Transaction_CHAINCODE_QUERY
		}
		tx, err = pb.NewChaincodeExecute(spec, uuid, t)
		if nil != err {
			return nil, err
		}
	}
	return tx, nil
}

// Invoke performs the supplied invocation on the specified chaincode through a transaction
func (d *Devops) Invoke(ctx context.Context, chaincodeInvocationSpec *pb.ChaincodeInvocationSpec) (*pb.Response, error) {
	return d.invokeOrQuery(ctx, chaincodeInvocationSpec, true)
}

// Query performs the supplied query on the specified chaincode through a transaction
func (d *Devops) Query(ctx context.Context, chaincodeInvocationSpec *pb.ChaincodeInvocationSpec) (*pb.Response, error) {
	return d.invokeOrQuery(ctx, chaincodeInvocationSpec, false)
}

// CheckSpec to see if chaincode resides within current package capture for language.
func CheckSpec(spec *pb.ChaincodeSpec) error {
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

	return nil
}

func checkGolangSpec(spec *pb.ChaincodeSpec) error {
	url, err := url.Parse(spec.ChaincodeID.Path)
	if err != nil || url == nil {
		return fmt.Errorf("invalid path: %s", err)
	}

	//we have no real good way of checking existence of remote urls except by downloading and testin
	//which we do later anyway. But we *can* - and *should* - test for existence of local paths.
	//Treat empty scheme as a local filesystem path
	if url.Scheme == "" {
		pathToCheck := filepath.Join(os.Getenv("GOPATH"), "src", spec.ChaincodeID.Path)
		exists, err := pathExists(pathToCheck)
		if err != nil {
			return fmt.Errorf("Error validating chaincode path: %s", err)
		}
		if !exists {
			return fmt.Errorf("Path to chaincode does not exist: %s", spec.ChaincodeID.Path)
		}
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

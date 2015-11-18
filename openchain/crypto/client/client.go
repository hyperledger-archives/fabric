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

package client

import (
	"errors"
	pb "github.com/openblockchain/obc-peer/protos"
)

// Errors

var ErrRegistrationRequired error = errors.New("Client Not Registered to the Membership Service.")
var ErrModuleNotInitialized error = errors.New("Client Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Client Security Module Already Initilized.")

// Public Struct

type Client struct {
	isInitialized bool
}

// Public Methods

// Register is used to register this client to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (client *Client) Register(userId, pwd string) error {
	return nil
}

// Init initializes this client by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (client *Client) Init() error {
	if client.isInitialized {
		return ErrModuleAlreadyInitialized
	}

	client.isInitialized = true
	return nil
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *Client) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *pb.ChaincodeDeploymentSpec, uuid string) (*pb.Transaction, error) {
	if !client.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	return pb.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, uuid)
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *Client) NewChaincodeInvokeTransaction(chaincodeInvocation *pb.ChaincodeInvocationSpec, uuid string) (*pb.Transaction, error) {
	if !client.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	return pb.NewChaincodeExecute(chaincodeInvocation, uuid, pb.Transaction_CHAINCODE_EXECUTE)
}

// Private Methods

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *Client) checkTransaction(*pb.Transaction) error {
	if !client.isInitialized {
		return ErrModuleNotInitialized
	}

	return nil
}

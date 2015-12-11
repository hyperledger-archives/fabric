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
	"fmt"
	"github.com/op/go-logging"
	obc "github.com/openblockchain/obc-peer/protos"
	"sync"
)

// Errors

var (
	ErrRegistrationRequired        error = errors.New("Client Not Registered to the Membership Service.")
	ErrModuleNotInitialized        error = errors.New("Client Security Module Not Initilized.")
	ErrModuleAlreadyInitialized    error = errors.New("Client Security Module Already Initilized.")
	ErrTransactionMissingCert      error = errors.New("Transaction missing certificate or signature.")
	ErrInvalidTransactionSignature error = errors.New("Invalid Transaction signature.")
	ErrModuleAlreadyRegistered     error = errors.New("Already registered.")
	ErrModuleNotRegisteredYet      error = errors.New("Not registered yet.")
	ErrInvalidClientReference      error = errors.New("Invalid client reference.")
)

// Private Variables

var (
	// Map of initialized clients
	clients map[string]Client = make(map[string]Client)

	// Sync
	mutex sync.Mutex
)

// Log

var log = logging.MustGetLogger("CRYPTO.CLIENT")

// Public Interfaces

type Client interface {
	GetID() string

	// NewChaincodeDeployTransaction is used to deploy chaincode.
	NewChaincodeDeployTransaction(chainletDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
	NewChaincodeInvokeTransaction(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)
}

// Public Methods

func Register(id, enrollID, enrollPWD string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Registering [%s,%s] with id [%s]...", enrollID, enrollPWD, id)

	if clients[id] != nil {
		return ErrModuleAlreadyRegistered
	}

	client := new(clientImpl)
	if err := client.Register(id, enrollID, enrollPWD); err != nil {
		return err
	}
	client.Close()

	log.Info("Registering [%s,%s] with id [%s]...done!", enrollID, enrollPWD, id)

	return nil
}

func Init(id string) (Client, error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Initializing [%s]...", id)

	if clients[id] != nil {
		log.Info("Client already initiliazied [%s].", id)

		return clients[id], nil
	}

	client := new(clientImpl)
	if err := client.Init(id); err != nil {
		log.Error("Failed initialization [%s]: %s", id, err)
		return nil, err
	}

	clients[id] = client
	log.Info("Initializing [%s]...done!", id)

	return client, nil
}

func Close(client Client) error {
	mutex.Lock()
	defer mutex.Unlock()

	id := client.GetID()
	defer delete(clients, id)

	log.Info("Closing client [%s]...", id)

	if _, ok := clients[id]; !ok {
		return ErrInvalidClientReference
	}

	err := clients[id].(*clientImpl).Close()

	log.Info("Closing client [%s]...done!", id)
	return err
}

func CloseAll() (bool, []error) {
	mutex.Lock()
	defer mutex.Unlock()

	errs := make([]error, len(clients))
	for _, value := range clients {
		append(errs, Close(value))
	}

	return len(errs) != 0, errs
}

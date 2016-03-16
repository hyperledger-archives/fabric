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

package crypto

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type clientImpl struct {
	*nodeImpl

	isInitialized bool

	// Chain
	chainPublicKey ecies.PublicKey
	queryStateKey  []byte

	// TCA KDFKey
	tCertOwnerKDFKey []byte
	tCertPool        tCertPool
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *clientImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next available (not yet used) transaction certificate
	tCert, err := client.tCertPool.GetNextTCert()
	if err != nil {
		client.error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeDeployUsingTCert(chaincodeDeploymentSpec, uuid, tCert, nil)
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *clientImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next available (not yet used) transaction certificate
	tCertHandler, err := client.tCertPool.GetNextTCert()
	if err != nil {
		client.error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeExecuteUsingTCert(chaincodeInvocation, uuid, tCertHandler, nil)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (client *clientImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next available (not yet used) transaction certificate
	tCertHandler, err := client.tCertPool.GetNextTCert()
	if err != nil {
		client.error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeQueryUsingTCert(chaincodeInvocation, uuid, tCertHandler, nil)
}

// GetEnrollmentCertHandler returns a CertificateHandler whose certificate is the enrollment certificate
func (client *clientImpl) GetEnrollmentCertificateHandler() (CertificateHandler, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Return the handler
	handler := &eCertHandlerImpl{}
	err := handler.init(client)
	if err != nil {
		client.error("Failed getting handler [%s].", err.Error())
		return nil, err
	}

	return handler, nil
}

// GetTCertHandlerNext returns a CertificateHandler whose certificate is the next available TCert
func (client *clientImpl) GetTCertificateHandlerNext() (CertificateHandler, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next TCert
	tCert, err := client.tCertPool.GetNextTCert()
	if err != nil {
		client.error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Return the handler
	handler := &tCertHandlerImpl{}
	err = handler.init(client, tCert)
	if err != nil {
		client.error("Failed getting handler [%s].", err.Error())
		return nil, err
	}

	return handler, nil
}

// GetTCertHandlerFromDER returns a CertificateHandler whose certificate is the one passed
func (client *clientImpl) GetTCertificateHandlerFromDER(tCertDER []byte) (CertificateHandler, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Validate the transaction certificate
	tCert, err := client.getTCertFromExternalDER(tCertDER)
	if err != nil {
		client.warning("Failed validating transaction certificate [%s].", err)

		return nil, err
	}

	// Return the handler
	handler := &tCertHandlerImpl{}
	err = handler.init(client, tCert)
	if err != nil {
		client.error("Failed getting handler [%s].", err.Error())
		return nil, err
	}

	return handler, nil
}

func (client *clientImpl) register(id string, pwd []byte, enrollID, enrollPWD string) (err error) {
	if client.isInitialized {
		client.error("Registering [%s]...done! Initialization already performed", id)

		return
	}

	// Register node
	if err = client.nodeImpl.register(NodeClient, id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] [%s].", enrollID, err.Error())
		return
	}

	client.info("Register crypto engine...")
	err = client.registerCryptoEngine()
	if err != nil {
		log.Error("Failed registering crypto engine [%s] [%s].", enrollID, err.Error())
		return
	}
	client.info("Register crypto engine...done.")

	return
}

func (client *clientImpl) init(id string, pwd []byte) error {
	if client.isInitialized {
		client.info("Already initializaed.")

		return nil
	}

	// Register node
	if err := client.nodeImpl.init(NodeClient, id, pwd); err != nil {
		return err
	}

	// Initialize keystore
	client.debug("Init keystore...")
	err := client.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			client.error("Keystore already initialized.")
		} else {
			client.error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	client.debug("Init keystore...done.")

	// Init crypto engine
	err = client.initCryptoEngine()
	if err != nil {
		client.error("Failed initiliazing crypto engine [%s].", err.Error())
		return err
	}

	// initialized
	client.isInitialized = true

	client.debug("Initialization...done.")

	return nil
}

func (client *clientImpl) close() (err error) {
	if client.tCertPool != nil {
		if err = client.tCertPool.Stop(); err != nil {
			client.debug("Failed closing TCertPool [%s]", err)
		}
	}

	if err = client.nodeImpl.close(); err != nil {
		client.debug("Failed closing node [%s]", err)
	}
	return
}

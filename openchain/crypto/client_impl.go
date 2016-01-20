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
	"crypto/aes"
	"crypto/cipher"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type clientImpl struct {
	node *nodeImpl

	isInitialized bool

	// TCA KDFKey
	tCertOwnerKDFKey []byte
}

func (client *clientImpl) GetName() string {
	return client.node.GetName()
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *clientImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next available (not yet used) transaction certificate
	rawTCert, err := client.getNextTCert()
	if err != nil {
		client.node.log.Error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeDeployUsingTCert(chaincodeDeploymentSpec, uuid, rawTCert, nil)
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *clientImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next available (not yet used) transaction certificate
	rawTCert, err := client.getNextTCert()
	if err != nil {
		client.node.log.Error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeExecuteUsingTCert(chaincodeInvocation, uuid, rawTCert, nil)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (client *clientImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Get next available (not yet used) transaction certificate
	rawTCert, err := client.getNextTCert()
	if err != nil {
		client.node.log.Error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeQueryUsingTCert(chaincodeInvocation, uuid, rawTCert, nil)
}

// DecryptQueryResult is used to decrypt the result of a query transaction
func (client *clientImpl) DecryptQueryResult(queryTx *obc.Transaction, ct []byte) ([]byte, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	queryKey := utils.HMACTruncated(client.node.enrollChainKey, append([]byte{6}, queryTx.Nonce...), utils.AESKeyLength)
	//	client.node.log.Info("QUERY Decrypting with key: ", utils.EncodeBase64(queryKey))

	if len(ct) <= utils.NonceSize {
		return nil, utils.ErrDecrypt
	}

	c, err := aes.NewCipher(queryKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	copy(nonce, ct)

	out, err := gcm.Open(nil, nonce, ct[gcm.NonceSize():], nil)
	if err != nil {
		client.node.log.Error("Failed decrypting query result [%s].", err.Error())
		return nil, utils.ErrDecrypt
	}
	return out, nil
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
		client.node.log.Error("Failed getting handler [%s].", err.Error())
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

	client.node.log.Info("Getting a CertificateHandler for the next available TCert...")

	// Get next TCert
	tCertDER, err := client.getNextTCert()
	if err != nil {
		client.node.log.Error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Return the handler
	handler := &tCertHandlerImpl{}
	err = handler.initDER(client, tCertDER)
	if err != nil {
		client.node.log.Error("Failed getting handler [%s].", err.Error())
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
	
	client.node.log.Info("Getting a CertificateHandler for TCert [%s]", utils.EncodeBase64(tCertDER))

	// Validate the transaction certificate
	tCert, err := client.validateTCert(tCertDER)
	if err != nil {
		client.node.log.Warning("Failed validating transaction certificate [%s].", err)

		return nil, err
	}

	// Return the handler
	handler := &tCertHandlerImpl{}
	err = handler.initX509(client, tCert)
	if err != nil {
		client.node.log.Error("Failed getting handler [%s].", err.Error())
		return nil, err
	}

	return handler, nil
}

func (client *clientImpl) register(name string, pwd []byte, enrollID, enrollPWD string) error {
	if client.isInitialized {
		client.node.log.Error("Registering [%s]...done! Initialization already performed", name)

		return nil
	}

	// Register node
	node := new(nodeImpl)
	if err := node.register("client", name, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] [%s].", enrollID, err.Error())
		return err
	}

	client.node = node

	return nil
}

func (client *clientImpl) init(id string, pwd []byte) error {
	if client.isInitialized {
		client.node.log.Info("Already initializaed.")

		return nil
	}

	// Register node
	var node *nodeImpl
	if client.node != nil {
		node = client.node
	} else {
		node = new(nodeImpl)
	}
	if err := node.init("client", id, pwd); err != nil {
		return err
	}
	client.node = node

	// Initialize keystore
	client.node.log.Info("Init keystore...")
	err := client.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			client.node.log.Error("Keystore already initialized.")
		} else {
			client.node.log.Error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	client.node.log.Info("Init keystore...done.")

	// Init crypto engine
	err = client.initCryptoEngine()
	if err != nil {
		client.node.log.Error("Failed initiliazing crypto engine [%s].", err.Error())
		return err
	}

	// initialized
	client.isInitialized = true

	client.node.log.Info("Initialization...done.")

	return nil
}

func (client *clientImpl) close() error {
	if client.node != nil {
		return client.node.close()
	}
	return nil
}

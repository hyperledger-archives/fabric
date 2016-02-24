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

	// TCert related fields
	tCertOwnerKDFKey []byte
	tCertPool        tCertPool
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
	tCert, err := client.tCertPool.GetNextTCert()
	if err != nil {
		client.node.error("Failed getting next transaction certificate [%s].", err.Error())
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
		client.node.error("Failed getting next transaction certificate [%s].", err.Error())
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
		client.node.error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Create Transaction
	return client.newChaincodeQueryUsingTCert(chaincodeInvocation, uuid, tCertHandler, nil)
}

// DecryptQueryResult is used to decrypt the result of a query transaction
func (client *clientImpl) DecryptQueryResult(queryTx *obc.Transaction, ct []byte) ([]byte, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	queryKey := utils.HMACTruncated(client.node.enrollChainKey, append([]byte{6}, queryTx.Nonce...), utils.AESKeyLength)
	//	client.node.info("QUERY Decrypting with key: ", utils.EncodeBase64(queryKey))

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
		client.node.error("Failed decrypting query result [%s].", err.Error())
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
		client.node.error("Failed getting handler [%s].", err.Error())
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
		client.node.error("Failed getting next transaction certificate [%s].", err.Error())
		return nil, err
	}

	// Return the handler
	handler := &tCertHandlerImpl{}
	err = handler.init(client, tCert)
	if err != nil {
		client.node.error("Failed getting handler [%s].", err.Error())
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
		client.node.warning("Failed validating transaction certificate [%s].", err)

		return nil, err
	}

	// Return the handler
	handler := &tCertHandlerImpl{}
	err = handler.init(client, tCert)
	if err != nil {
		client.node.error("Failed getting handler [%s].", err.Error())
		return nil, err
	}

	return handler, nil
}

func (client *clientImpl) register(name string, pwd []byte, enrollID, enrollPWD string) error {
	if client.isInitialized {
		client.node.error("Registering [%s]...done! Initialization already performed", name)

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
		client.node.info("Already initializaed.")

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
	client.node.debug("Init keystore...")
	err := client.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			client.node.error("Keystore already initialized.")
		} else {
			client.node.error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	client.node.debug("Init keystore...done.")

	// Init crypto engine
	err = client.initCryptoEngine()
	if err != nil {
		client.node.error("Failed initiliazing crypto engine [%s].", err.Error())
		return err
	}

	// initialized
	client.isInitialized = true

	client.node.debug("Initialization...done.")

	return nil
}

func (client *clientImpl) close() (err error) {
	if client.tCertPool != nil {
		if err = client.tCertPool.Stop(); err != nil {
			client.node.debug("Failed closing TCertPool [%s]", err)
		}
	}

	if client.node != nil {
		if err = client.node.close(); err != nil {
			client.node.debug("Failed closing node [%s]", err)
		}
	}
	return
}

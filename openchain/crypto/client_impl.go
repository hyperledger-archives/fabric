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
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type clientImpl struct {
	node *nodeImpl

	isInitialized bool
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *clientImpl) NewChaincodeDeployTransaction(chainletDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeDeployTransaction(chainletDeploymentSpec, uuid)
	if err != nil {
		client.node.log.Error("Failed creating new transaction %s:", err)
		return nil, err
	}

	// TODO: implement like this.
	// getNextTCert returns only a TCert
	// Then, invoke signWithTCert to sign the signature

	// Get next available (not yet used) transaction certificate
	// with the relative signing key.
	rawTCert, signKey, err := client.getNextTCert()
	if err != nil {
		client.node.log.Error("Failed getting next transaction certificate %s:", err)
		return nil, err
	}

	// Append the certificate to the transaction
	client.node.log.Info("Appending certificate %s", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.node.log.Error("Failed marshaling tx %s:", err)
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.node.log.Info("Signing tx %s", utils.EncodeBase64(rawTx))
	rawSignature, err := client.node.sign(signKey, rawTx)
	if err != nil {
		client.node.log.Error("Failed creating signature %s:", err)
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.node.log.Info("Appending signature %s", utils.EncodeBase64(rawSignature))

	return tx, nil
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *clientImpl) NewChaincodeInvokeTransaction(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		client.node.log.Error("Failed creating new transaction %s:", err)
		return nil, err
	}

	// TODO: implement like this.
	// getNextTCert returns only a TCert
	// Then, invoke signWithTCert to sign the signature

	// Get next available (not yet used) transaction certificate
	// with the relative signing key.
	rawTCert, signKey, err := client.getNextTCert()
	if err != nil {
		client.node.log.Error("Failed getting next transaction certificate %s:", err)
		return nil, err
	}

	// Append the certificate to the transaction
	client.node.log.Info("Appending certificate %s", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.node.log.Error("Failed marshaling tx %s:", err)
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.node.log.Info("Signing tx %s", utils.EncodeBase64(rawTx))
	rawSignature, err := client.node.sign(signKey, rawTx)
	if err != nil {
		client.node.log.Error("Failed creating signature %s:", err)
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.node.log.Info("Appending signature %s", utils.EncodeBase64(rawSignature))

	return tx, nil
}

func (client *clientImpl) GetName() string {
	return client.node.GetName()
}

func (client *clientImpl) register(id string, pwd []byte, enrollID, enrollPWD string) error {
	if client.isInitialized {
		log.Error("Registering [%s]...done! Initialization already performed", enrollID)

		return nil
	}

	// Register node
	node := new(nodeImpl)
	if err := node.register("client", id, pwd, enrollID, enrollPWD); err != nil {
		return err
	}

	client.node = node
	client.isInitialized = true
	return nil
}

func (client *clientImpl) init(id string, pwd []byte) error {
	if client.isInitialized {
		client.node.log.Error("Already initializaed.")

		return nil
	}

	// Register node
	node := new(nodeImpl)
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
			client.node.log.Error("Failed initiliazing keystore %s", err)

			return err
		}
	}
	client.node.log.Info("Init keystore...done.")

	// initialized
	client.isInitialized = true

	client.node.log.Info("Initialization...done.")

	return nil
}

func (client *clientImpl) close() error {
	return client.node.close()
}

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *clientImpl) checkTransaction(tx *obc.Transaction) error {
	if !client.isInitialized {
		return utils.ErrNotInitialized
	}

	if tx.Cert == nil && tx.Signature == nil {
		return utils.ErrTransactionMissingCert
	}

	if tx.Cert != nil && tx.Signature != nil {
		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			client.node.log.Error("Failed unmarshalling cert %s:", err)
			return err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			client.node.log.Error("Failed marshaling tx %s:", err)
			return err
		}
		tx.Signature = signature

		// 2. Verify signature
		ver, err := client.node.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			client.node.log.Error("Failed marshaling tx %s:", err)
			return err
		}

		if ver {
			return nil
		}

		return utils.ErrInvalidTransactionSignature
	}

	return utils.ErrTransactionMissingCert
}

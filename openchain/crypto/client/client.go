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
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	_ "github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

// Errors

var ErrRegistrationRequired error = errors.New("Client Not Registered to the Membership Service.")
var ErrModuleNotInitialized error = errors.New("Client Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Client Security Module Already Initilized.")
var ErrTransactionMissingCert error = errors.New("Transaction missing certificate or signature.")
var ErrInvalidTransactionSignature error = errors.New("Invalid Transaction signature.")

// Log

var log = logging.MustGetLogger("CRYPTO.CLIENT")

// Public Structs

type Client struct {
	isInitialized bool

	id []byte

	rootsCertPool *x509.CertPool

	// Enrollment Certificate and private key
	enrollId      string
	enrollCert    *x509.Certificate
	enrollPrivKey *ecdsa.PrivateKey

	// Enrollment Chain
	enrollChainKey []byte
}

// Public Methods

// Register is used to register this client to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (client *Client) Register(userId, pwd string) error {
	log.Info("Registering user [%s]...", userId)

	if err := client.createKeyStorage(); err != nil {
		log.Error("Failed creating key storage: %s", err)

		return err
	}

	if err := client.retrieveECACertsChain(userId); err != nil {
		log.Error("Failed retrieveing ECA certs chain: %s", err)

		return err
	}

	if err := client.retrieveTCACertsChain(userId); err != nil {
		log.Error("Failed retrieveing ECA certs chain: %s", err)

		return err
	}

	if err := client.retrieveEnrollmentData(userId, pwd); err != nil {
		log.Error("Failed retrieveing enrollment data: %s", err)

		return err
	}

	log.Info("Registering user [%s]...done!", userId)

	return nil
}

func (client *Client) GetID() string {
	// TODO: shall we clone id?

	return client.enrollId
}

// Init initializes this client by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (client *Client) Init() error {
	log.Info("Initialization...")

	if client.isInitialized {
		log.Error("Already initializaed.")

		return ErrModuleAlreadyInitialized
	}

	// Init Conf
	if err := initConf(); err != nil {
		log.Error("Invalid configuration: %s", err)

		return err
	}

	// Initialize DB
	log.Info("Init DB...")
	err := initDB()
	if err != nil {
		if err != ErrDBAlreadyInitialized {
			log.Error("DB already initialized.")
		} else {
			log.Error("Failed initiliazing DB %s", err)

			return err
		}
	}
	log.Info("Init DB...done.")

	// Init crypto engine
	err = client.initCryptoEngine()
	if err != nil {
		log.Error("Failed initiliazing crypto engine %s", err)
		return err
	}

	// initialized
	client.isInitialized = true

	log.Info("Initialization...done.")

	return nil
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *Client) NewChaincodeDeployTransaction(chainletDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeDeployTransaction(chainletDeploymentSpec, uuid)
	if err != nil {
		log.Error("Failed creating new transaction %s:", err)
		return nil, err
	}

	// TODO: confidentiality

	// 1. set confidentiality level and nonce
	tx.ConfidentialityLevel = obc.Transaction_CHAINCODE_CONFIDENTIAL
	tx.Nonce, err = utils.GetRandomBytes(32) // TODO: magic number?
	if err != nil {
		log.Error("Failed creating nonce %s:", err)
		return nil, err
	}

	// 2. encrypt and set payload
	ct, err := client.encryptPayload(tx)
	if err != nil {
		log.Error("Failed encrypting payload %s:", err)
		return nil, err
	}
	tx.Payload = ct
	log.Info("Payload %s", utils.EncodeBase64(tx.Payload))

	// TODO: Sign the transaction

	// Implement like this: getNextTCert returns only a TCert
	// Then, invoke signWithTCert to sign the signature

	// Get next available (not yet used) transaction certificate
	// with the relative signing key.
	rawTCert, signKey, err := client.getNextTCert()
	if err != nil {
		log.Error("Failed getting next transaction certificate %s:", err)
		return nil, err
	}

	// Append the certificate to the transaction
	log.Info("Appending certificate %s", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		log.Error("Failed marshaling tx %s:", err)
		return nil, err
	}

	// 2. Sign rawTx and check signature
	log.Info("Signing tx %s", utils.EncodeBase64(rawTx))
	rawSignature, err := client.sign(signKey, rawTx)
	if err != nil {
		log.Error("Failed creating signature %s:", err)
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	log.Info("Appending signature %s", utils.EncodeBase64(rawSignature))

	return tx, nil
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *Client) NewChaincodeInvokeTransaction(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		log.Error("Failed creating new transaction %s:", err)
		return nil, err
	}

	// TODO: confidentiality

	// 1. set confidentiality level and nonce
	tx.ConfidentialityLevel = obc.Transaction_CHAINCODE_CONFIDENTIAL
	tx.Nonce, err = utils.GetRandomBytes(32) // TODO: magic number?
	if err != nil {
		log.Error("Failed creating nonce %s:", err)
		return nil, err
	}

	// 2. encrypt and set payload
	ct, err := client.encryptPayload(tx)
	if err != nil {
		log.Error("Failed encrypting payload %s:", err)
		return nil, err
	}
	tx.Payload = ct

	// TODO: Sign the transaction

	// Implement like this: getNextTCert returns only a TCert
	// Then, invoke signWithTCert to sign the signature

	// Get next available (not yet used) transaction certificate
	// with the relative signing key.
	rawTCert, signKey, err := client.getNextTCert()
	if err != nil {
		log.Error("Failed getting next transaction certificate %s:", err)
		return nil, err
	}

	// Append the certificate to the transaction
	log.Info("Appending certificate %s", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		log.Error("Failed marshaling tx %s:", err)
		return nil, err
	}

	// 2. Sign rawTx and check signature
	log.Info("Signing tx %s", utils.EncodeBase64(rawTx))
	rawSignature, err := client.sign(signKey, rawTx)
	if err != nil {
		log.Error("Failed creating signature %s:", err)
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	log.Info("Appending signature %s", utils.EncodeBase64(rawSignature))

	return tx, nil
}

func (client *Client) Close() error {
	getDBHandle().CloseDB()

	return nil
}

// Private Methods

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *Client) checkTransaction(tx *obc.Transaction) error {
	if !client.isInitialized {
		return ErrModuleNotInitialized
	}

	if tx.Cert == nil && tx.Signature == nil {
		return ErrTransactionMissingCert
	}

	if tx.Cert != nil && tx.Signature != nil {
		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			log.Error("Failed unmarshalling cert %s:", err)
			return err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			log.Error("Failed marshaling tx %s:", err)
			return err
		}
		tx.Signature = signature

		// 2. Verify signature
		ver, err := client.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			log.Error("Failed marshaling tx %s:", err)
			return err
		}

		if ver {
			return nil
		}

		return ErrInvalidTransactionSignature
	}

	return ErrTransactionMissingCert
}

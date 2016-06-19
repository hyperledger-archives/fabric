/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

type peerImpl struct {
	*nodeImpl

	nodeEnrollmentCertificatesMutex sync.RWMutex
	nodeEnrollmentCertificates      map[string]*x509.Certificate

	isInitialized bool
}

// Public methods

// GetID returns this peer's identifier
func (peer *peerImpl) GetID() []byte {
	return utils.Clone(peer.id)
}

// GetEnrollmentID returns this peer's enrollment id
func (peer *peerImpl) GetEnrollmentID() string {
	return peer.enrollID
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification).
func (peer *peerImpl) TransactionPreValidation(tx *obc.Transaction) (*obc.Transaction, error) {
	if !peer.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	//	peer.debug("Pre validating [%s].", tx.String())
	peer.Debugf("Tx confdential level [%s].", tx.ConfidentialityLevel.String())

	if tx.Cert != nil && tx.Signature != nil {
		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := primitives.DERToX509Certificate(tx.Cert)
		if err != nil {
			peer.Errorf("TransactionPreExecution: failed unmarshalling cert [%s].", err.Error())
			return tx, err
		}

		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			peer.Errorf("TransactionPreExecution: failed marshaling tx [%s].", err.Error())
			return tx, err
		}
		tx.Signature = signature

		// 2. Verify signature
		ok, err := peer.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			peer.Errorf("TransactionPreExecution: failed marshaling tx [%s].", err.Error())
			return tx, err
		}

		if !ok {
			return tx, utils.ErrInvalidTransactionSignature
		}
	} else {
		if tx.Cert == nil {
			return tx, utils.ErrTransactionCertificate
		}

		if tx.Signature == nil {
			return tx, utils.ErrTransactionSignature
		}
	}

	return tx, nil
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification). If this is the case,
// the method prepares the transaction to be executed.
func (peer *peerImpl) TransactionPreExecution(tx *obc.Transaction) (*obc.Transaction, error) {
	return nil, utils.ErrNotImplemented
}

// Sign signs msg with this validator's signing key and outputs
// the signature if no error occurred.
func (peer *peerImpl) Sign(msg []byte) ([]byte, error) {
	return peer.signWithEnrollmentKey(msg)
}

// Verify checks that signature if a valid signature of message under vkID's verification key.
// If the verification succeeded, Verify returns nil meaning no error occurred.
// If vkID is nil, then the signature is verified against this validator's verification key.
func (peer *peerImpl) Verify(vkID, signature, message []byte) error {
	if len(vkID) == 0 {
		return fmt.Errorf("Invalid peer id. It is empty.")
	}
	if len(signature) == 0 {
		return fmt.Errorf("Invalid signature. It is empty.")
	}
	if len(message) == 0 {
		return fmt.Errorf("Invalid message. It is empty.")
	}

	cert, err := peer.getEnrollmentCert(vkID)
	if err != nil {
		peer.Errorf("Failed getting enrollment cert for [% x]: [%s]", vkID, err)

		return err
	}

	vk := cert.PublicKey.(*ecdsa.PublicKey)

	ok, err := peer.verify(vk, message, signature)
	if err != nil {
		peer.Errorf("Failed verifying signature for [% x]: [%s]", vkID, err)

		return err
	}

	if !ok {
		peer.Errorf("Failed invalid signature for [% x]", vkID)

		return utils.ErrInvalidSignature
	}

	return nil
}

func (peer *peerImpl) GetStateEncryptor(deployTx, invokeTx *obc.Transaction) (StateEncryptor, error) {
	return nil, utils.ErrNotImplemented
}

func (peer *peerImpl) GetTransactionBinding(tx *obc.Transaction) ([]byte, error) {
	return primitives.Hash(append(tx.Cert, tx.Nonce...)), nil
}

// Private methods

func (peer *peerImpl) register(eType NodeType, name string, pwd []byte, enrollID, enrollPWD string) error {
	if peer.isInitialized {
		peer.Errorf("Registering [%s]...done! Initialization already performed", enrollID)

		return utils.ErrAlreadyInitialized
	}

	// Register node
	if err := peer.nodeImpl.register(eType, name, pwd, enrollID, enrollPWD); err != nil {
		peer.Errorf("Failed registering [%s]: [%s]", enrollID, err)
		return err
	}

	return nil
}

func (peer *peerImpl) init(eType NodeType, id string, pwd []byte) error {
	if peer.isInitialized {
		return utils.ErrAlreadyInitialized
	}

	// Register node
	if err := peer.nodeImpl.init(eType, id, pwd); err != nil {
		return err
	}

	// Initialize keystore
	peer.Debug("Init keystore...")
	err := peer.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			peer.Error("Keystore already initialized.")
		} else {
			peer.Errorf("Failed initiliazing keystore [%s].", err)

			return err
		}
	}
	peer.Debug("Init keystore...done.")

	// initialized
	peer.isInitialized = true

	// EnrollCerts
	peer.nodeEnrollmentCertificates = make(map[string]*x509.Certificate)

	return nil
}

func (peer *peerImpl) close() error {
	return peer.nodeImpl.close()
}

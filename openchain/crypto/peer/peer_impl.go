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

package peer

import (
	"crypto/x509"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

// Public Struct

type peerImpl struct {
	isInitialized bool

	// Configuration
	conf *configuration

	// Logging
	log *logging.Logger

	// keyStore
	ks *keyStore

	// Certs
	rootsCertPool *x509.CertPool

	// 48-bytes identifier
	id []byte

	// Enrollment Certificate and private key
	enrollID      string
	enrollCert    *x509.Certificate
	enrollPrivKey interface{}
	enrollCerts   map[string]*x509.Certificate
}

// Public Methods

// Register is used to register this peer to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the peer
// is first deployed.
func (peer *peerImpl) Register(id string, pwd []byte, enrollID, enrollPWD string) error {
	if peer.isInitialized {
		log.Error("Registering [%s]...done! Initialization already performed", enrollID)

		return nil
	}

	// Init Conf
	if err := peer.initConfiguration(id); err != nil {
		log.Error("Failed ini configuration [%s]: %s", enrollID, err)

		return err
	}

	// Start registration
	peer.log.Info("Registering [%s]...", enrollID)

	if peer.isRegistered() {
		peer.log.Error("Registering [%s]...done! Registration already performed", enrollID)
	} else {
		if err := peer.createKeyStorage(); err != nil {
			peer.log.Error("Failed creating key storage: %s", err)

			return err
		}

		if err := peer.retrieveECACertsChain(enrollID); err != nil {
			peer.log.Error("Failed retrieveing ECA certs chain: %s", err)

			return err
		}

		if err := peer.retrieveTCACertsChain(enrollID); err != nil {
			peer.log.Error("Failed retrieveing ECA certs chain: %s", err)

			return err
		}

		if err := peer.retrieveEnrollmentData(enrollID, enrollPWD); err != nil {
			peer.log.Error("Failed retrieveing enrollment data: %s", err)

			return err
		}
	}

	peer.log.Info("Registering [%s]...done!", enrollID)

	return nil
}

// Init initializes this peer by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the peer is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (peer *peerImpl) Init(id string, pwd []byte) error {
	if peer.isInitialized {
		peer.log.Error("Already initializaed.")

		return nil
	}

	// Init Conf
	if err := peer.initConfiguration(id); err != nil {
		return err
	}

	if !peer.isRegistered() {
		return utils.ErrRegistrationRequired
	}

	// Initialize keystore
	peer.log.Info("Init keystore...")
	// TODO: password support
	err := peer.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			peer.log.Error("Keystore already initialized.")
		} else {
			peer.log.Error("Failed initiliazing keystore %s", err)

			return err
		}
	}
	peer.log.Info("Init keystore...done.")

	// Init crypto engine
	err = peer.initCryptoEngine()
	if err != nil {
		peer.log.Error("Failed initiliazing crypto engine %s", err)
		return err
	}

	peer.log.Info("Initialization...done.")

	// Initialisation complete
	peer.isInitialized = true

	return nil
}

func (peer *peerImpl) GetName() string {
	return peer.conf.id
}

// GetID returns this peer's identifier
func (peer *peerImpl) GetID() []byte {
	// Clone id to avoid exposure of internal data structure
	clone := make([]byte, len(peer.id))
	copy(clone, peer.id)

	return clone
}

// GetEnrollmentID returns this peer's enroolment id
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

	if tx.Cert != nil && tx.Signature != nil {
		peer.log.Info("TransactionPreValidation: executing...")

		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			peer.log.Error("TransactionPreExecution: failed unmarshalling cert %s:", err)
			return tx, err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			peer.log.Error("TransactionPreExecution: failed marshaling tx %s:", err)
			return tx, err
		}
		tx.Signature = signature

		// 2. Verify signature
		ok, err := peer.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			peer.log.Error("TransactionPreExecution: failed marshaling tx %s:", err)
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

func (peer *peerImpl) Close() error {
	// Close keystore
	var err error

	if peer.ks != nil {
		err = peer.ks.Close()
	}

	return err
}

// Private Methods

func (peer *peerImpl) initCryptoEngine() error {
	peer.log.Info("Initialing Crypto Engine...")

	peer.rootsCertPool = x509.NewCertPool()
	peer.enrollCerts = make(map[string]*x509.Certificate)

	// Load ECA certs chain
	if err := peer.loadECACertsChain(); err != nil {
		return err
	}

	// Load TCA certs chain
	if err := peer.loadTCACertsChain(); err != nil {
		return err
	}

	// Load enrollment secret key
	// TODO: finalize encrypted pem support
	if err := peer.loadEnrollmentKey(nil); err != nil {
		return err
	}

	// Load enrollment certificate and set peer ID
	if err := peer.loadEnrollmentCertificate(); err != nil {
		return err
	}

	// Load enrollment id
	if err := peer.loadEnrollmentID(); err != nil {
		return err
	}

	peer.log.Info("Initialing Crypto Engine...done!")

	return nil
}

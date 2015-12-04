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
	"crypto/rand"
	"crypto/x509"
	"errors"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	pb "github.com/openblockchain/obc-peer/protos"
)

// Errors

var ErrRegistrationRequired error = errors.New("Peer Not Registered to the Membership Service.")
var ErrModuleNotInitialized = errors.New("Peer Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Peer Security Module Already Initilized.")

// Log

var peerLogger = logging.MustGetLogger("CRYPTO.PEER")

// Public Struct

type Peer struct {
	isInitialized bool

	rootsCertPool *x509.CertPool

	// 48-bytes identifier
	id []byte

	// Enrollment Certificate and private key
	enrollCert    *x509.Certificate
	enrollPrivKey interface{}
}

// Public Methods

// Register is used to register this peer to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (peer *Peer) Register(userId, pwd string) error {
	return nil
}

// Init initializes this peer by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (peer *Peer) Init() error {
	if peer.isInitialized {
		return ErrModuleAlreadyInitialized
	}

	// Init field

	// id is initialized to a random value. Later on,
	// id will be initialized as the hash of the enrollment certificate
	peer.id = make([]byte, 48)
	_, err := rand.Read(peer.id)
	if err != nil {
		return err
	}

	// Initialisation complete
	peer.isInitialized = true

	return nil
}

// GetID returns this validator's identifier
func (peer *Peer) GetID() []byte {
	// Clone id to avoid exposure of internal data structure
	clone := make([]byte, len(peer.id))
	copy(clone, peer.id)

	return clone
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification)
func (peer *Peer) TransactionPreValidation(tx *pb.Transaction) (*pb.Transaction, error) {
	if !peer.isInitialized {
		return nil, ErrModuleNotInitialized
	}

	return tx, nil
}

func (peer *Peer) initCryptoEngine() error {
	// Init certificate pool
	peerLogger.Info("Initialing roots cert pool...")

	//	validator.rootsCertPool = x509.NewCertPool()
	//	validator.rootsCertPool.AppendCertsFromPEM([]byte(tcaChainCert))

	// TODO: Load enrollment secret key
	peerLogger.Info("Loading enrollment key...")
	var err error
	peer.enrollPrivKey, err = utils.NewECDSAKey()

	return err
}

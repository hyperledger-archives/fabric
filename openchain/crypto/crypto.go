package crypto

import (
	obc "github.com/openblockchain/obc-peer/protos"
)

// Public Interfaces

// NodeType represents the node's type
type NodeType int32

const (
	// NodeClient a client
	NodeClient NodeType = 0
	// NodePeer a peer
	NodePeer NodeType = 1
	// NodeValidator a validator
	NodeValidator NodeType = 2
)

// Node represents a crypto object having a name
type Node interface {

	// GetType returns this entity's name
	GetType() NodeType

	// GetName returns this entity's name
	GetName() string
}

// Client is an entity able to deploy and invoke chaincode
type Client interface {
	Node

	// NewChaincodeDeployTransaction is used to deploy chaincode.
	NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeExecute is used to execute chaincode's functions.
	NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeQuery is used to query chaincode's functions.
	NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)

	// DecryptQueryResult is used to decrypt the result of a query transaction
	DecryptQueryResult(queryTx *obc.Transaction, result []byte) ([]byte, error)

	// GetEnrollmentCertHandler returns a CertificateHandler whose certificate is the enrollment certificate
	GetEnrollmentCertificateHandler() (CertificateHandler, error)

	// GetTCertHandlerNext returns a CertificateHandler whose certificate is the next available TCert
	GetTCertificateHandlerNext() (CertificateHandler, error)

	// GetTCertHandlerFromDER returns a CertificateHandler whose certificate is the one passed
	GetTCertificateHandlerFromDER(der []byte) (CertificateHandler, error)
}

// Peer is an entity able to verify transactions
type Peer interface {
	Node

	// GetID returns this peer's identifier
	GetID() []byte

	// GetEnrollmentID returns this peer's enrollment id
	GetEnrollmentID() string

	// TransactionPreValidation verifies that the transaction is
	// well formed with the respect to the security layer
	// prescriptions (i.e. signature verification).
	TransactionPreValidation(tx *obc.Transaction) (*obc.Transaction, error)

	// TransactionPreExecution verifies that the transaction is
	// well formed with the respect to the security layer
	// prescriptions (i.e. signature verification). If this is the case,
	// the method prepares the transaction to be executed.
	// TransactionPreExecution returns a clone of tx.
	TransactionPreExecution(tx *obc.Transaction) (*obc.Transaction, error)

	// Sign signs msg with this validator's signing key and outputs
	// the signature if no error occurred.
	Sign(msg []byte) ([]byte, error)

	// Verify checks that signature if a valid signature of message under vkID's verification key.
	// If the verification succeeded, Verify returns nil meaning no error occurred.
	// If vkID is nil, then the signature is verified against this validator's verification key.
	Verify(vkID, signature, message []byte) error

	// GetStateEncryptor returns a StateEncryptor linked to pair defined by
	// the deploy transaction and the execute transaction. Notice that,
	// executeTx can also correspond to a deploy transaction.
	GetStateEncryptor(deployTx, executeTx *obc.Transaction) (StateEncryptor, error)
}

// StateEncryptor is used to encrypt chaincode's state
type StateEncryptor interface {

	// Encrypt encrypts message msg
	Encrypt(msg []byte) ([]byte, error)

	// Decrypt decrypts ciphertext ct obtained
	// from a call of the Encrypt method.
	Decrypt(ct []byte) ([]byte, error)
}

// CertificateHandler exposes methods to deal with an ECert/TCert
type CertificateHandler interface {

	// GetCertificate returns the certificate's DER
	GetCertificate() []byte

	// Sign signs msg using the signing key corresponding to the certificate
	Sign(msg []byte) ([]byte, error)

	// Verify verifies msg using the verifying key corresponding to the certificate
	Verify(signature []byte, msg []byte) error

	// GetTransactionHandler returns a new transaction handler relative to this certificate
	GetTransactionHandler() (TransactionHandler, error)
}

// TransactionHandler represents a single transaction that can be named by the output of the GetBinding method.
// This transaction is linked to a single Certificate (TCert or ECert).
type TransactionHandler interface {

	// GetCertificateHandler returns the certificate handler relative to the certificate mapped to this transaction
	GetCertificateHandler() (CertificateHandler, error)

	// GetBinding returns a binding to the underlying transaction
	GetBinding() ([]byte, error)

	// NewChaincodeDeployTransaction is used to deploy chaincode
	NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeExecute is used to execute chaincode's functions
	NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeQuery is used to query chaincode's functions
	NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)
}

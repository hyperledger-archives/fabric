package crypto

import (
	obc "github.com/hyperledger/fabric/protos"
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

	// ConfidentialityProtocolVersion1_1 version 1.1
	ConfidentialityProtocolVersion1_1 string = "1.1"
	// ConfidentialityProtocolVersion1_2 version 1.2
	ConfidentialityProtocolVersion1_2 string = "1.2"
	// ConfidentialityProtocolVersion2 version 2.0
	ConfidentialityProtocolVersion2 string = "2.0"
)

// Node represents a crypto object having a type and name
type Node interface {

	// GetType returns this entity's type
	GetType() NodeType

	// GetName returns this entity's name
	GetName() string
}

// Client is a node that can deploy, execute and query chaincodes
type Client interface {
	Node

	// NewChaincodeDeployTransaction is used to deploy chaincode.
	// This call is equivalent to invoking GetTCertificateHandlerNext() first and then
	// NewChaincodeDeployTransaction on the handler.
	NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeExecute is used to execute chaincode's functions.
	// This call is equivalent to invoking GetTCertificateHandlerNext() first and then
	// NewChaincodeExecute on the handler.
	NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeQuery is used to query chaincode's functions.
	// This call is equivalent to invoking GetTCertificateHandlerNext() first and then
	// NewChaincodeQuery on the handler.
	NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)

	// DecryptQueryResult is used to decrypt the result of a query transaction
	DecryptQueryResult(queryTx *obc.Transaction, result []byte) ([]byte, error)

	// GetEnrollmentCertHandler returns a CertificateHandler whose certificate is the enrollment certificate
	GetEnrollmentCertificateHandler() (CertificateHandler, error)

	// GetTCertHandlerNext returns a CertificateHandler whose certificate is the next available TCert
	GetTCertificateHandlerNext() (CertificateHandler, error)

	// GetTCertHandlerFromDER returns a CertificateHandler whose certificate is the one passed
	GetTCertificateHandlerFromDER(der []byte) (CertificateHandler, error)

	// GetEncryptionKey returns the serialized version of this client's enrollment encryption key
	GetEncryptionKey() ([]byte, error)

	// ReadAttribute reads the attribute with name 'attributeName' from the der encoded x509.Certificate 'tcertder'.
	ReadAttribute(attributeName string, tcertder []byte) ([]byte, error)
}

// Peer is a node that verify transactions
type Peer interface {
	Node

	// GetID returns this peer's identifier
	GetID() []byte

	// GetEnrollmentID returns this peer's enrollment id
	GetEnrollmentID() string

	// GetChaincodeID returns tx's ChaincodeID
	GetChaincodeID(tx *obc.Transaction) (*obc.ChaincodeID, error)

	// TransactionPreValidation verifies that the transaction is
	// well formed with the respect to the security layer
	// prescriptions (i.e. signature verification).
	TransactionPreValidation(tx *obc.Transaction) (*obc.Transaction, error)

	// TransactionPreExecution verifies that the transaction is
	// well formed with the respect to the security layer
	// prescriptions (i.e. signature verification). If this is the case,
	// the method prepares the transaction to be executed.
	// TransactionPreExecution returns a clone of tx.
	TransactionPreExecution(deployTx, tx *obc.Transaction) (*obc.Transaction, error)

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

	// GetTransactionBinding returns the binding associated to tx.
	GetTransactionBinding(tx *obc.Transaction) ([]byte, error)
}

// StateEncryptor encrypts chaincode's states
type StateEncryptor interface {

	// Encrypt encrypts message msg
	Encrypt(msg []byte) ([]byte, error)

	// Decrypt decrypts ciphertext ct obtained
	// from a call of the Encrypt method.
	Decrypt(ct []byte) ([]byte, error)
}

// CertificateHandler handles an ECert or TCert
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

	// GetBinding returns a binding to the underlying transaction. It is used to name each transaction.
	GetBinding() ([]byte, error)

	// NewChaincodeDeployTransaction is used to deploy chaincode
	NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeExecute is used to execute chaincode's functions
	NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)

	// NewChaincodeQuery is used to query chaincode's functions
	NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error)
}

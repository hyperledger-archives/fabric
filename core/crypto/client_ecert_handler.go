package crypto

import (
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

type eCertHandlerImpl struct {
	client *clientImpl
}

// GetCertificate returns the TCert DER
func (handler *eCertHandlerImpl) GetCertificate() []byte {
	return utils.Clone(handler.client.enrollCert.Raw)
}

// Sign signs msg using the signing key corresponding to this TCert
func (handler *eCertHandlerImpl) Sign(msg []byte) ([]byte, error) {
	return handler.client.eCert.Sign(msg)
}

// Verify verifies msg using the verifying key corresponding to this TCert
func (handler *eCertHandlerImpl) Verify(signature []byte, msg []byte) error {
	err := handler.client.eCert.Verify(signature, msg)
	if err != nil {
		return err
	}
	return nil
}

// GetTransactionHandler returns the transaction handler relative to this certificate
func (handler *eCertHandlerImpl) GetTransactionHandler() (TransactionHandler, error) {
	txHandler := &eCertTransactionHandlerImpl{}
	err := txHandler.init(handler.client)
	if err != nil {
		handler.client.error("Failed getting transaction handler [%s]", err)

		return nil, err
	}

	return txHandler, nil
}

type eCertTransactionHandlerImpl struct {
	client *clientImpl

	nonce   []byte
	binding []byte
}

func (handler *eCertTransactionHandlerImpl) init(client *clientImpl) error {
	nonce, err := client.createTransactionNonce()
	if err != nil {
		client.error("Failed initiliazing transaction handler [%s]", err)

		return err
	}

	handler.client = client
	handler.nonce = nonce
	handler.binding = primitives.Hash(append(handler.client.enrollCert.Raw, handler.nonce...))

	return nil
}

// GetCertificateHandler returns the certificate handler relative to the certificate mapped to this transaction
func (handler *eCertTransactionHandlerImpl) GetCertificateHandler() (CertificateHandler, error) {
	return handler.client.GetEnrollmentCertificateHandler()
}

// GetBinding returns an Binding to the underlying transaction layer
func (handler *eCertTransactionHandlerImpl) GetBinding() ([]byte, error) {
	return utils.Clone(handler.binding), nil
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (handler *eCertTransactionHandlerImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newTransactionContext(handler, handler.client.eCert, uuid, handler.nonce).newChaincodeDeploy(chaincodeDeploymentSpec)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *eCertTransactionHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newTransactionContext(handler, handler.client.eCert, uuid, handler.nonce).newChaincodeExecute(chaincodeInvocation)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *eCertTransactionHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newTransactionContext(handler, handler.client.eCert, uuid, handler.nonce).newChaincodeQuery(chaincodeInvocation)
}

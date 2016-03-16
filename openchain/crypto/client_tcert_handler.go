package crypto

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type tCertHandlerImpl struct {
	client *clientImpl

	tCert tCert
}

type tCertTransactionHandlerImpl struct {
	tCertHandler *tCertHandlerImpl

	nonce   []byte
	binding []byte
}

func (handler *tCertHandlerImpl) init(client *clientImpl, tCert tCert) error {
	handler.client = client
	handler.tCert = tCert

	return nil
}

// GetCertificate returns the TCert DER
func (handler *tCertHandlerImpl) GetCertificate() []byte {
	return utils.Clone(handler.tCert.GetCertificate().Raw)
}

// Sign signs msg using the signing key corresponding to this TCert
func (handler *tCertHandlerImpl) Sign(msg []byte) ([]byte, error) {
	return handler.tCert.Sign(msg)
}

// Verify verifies msg using the verifying key corresponding to this TCert
func (handler *tCertHandlerImpl) Verify(signature []byte, msg []byte) error {
	return handler.tCert.Verify(signature, msg)
}

// GetTransactionHandler returns the transaction handler relative to this certificate
func (handler *tCertHandlerImpl) GetTransactionHandler() (TransactionHandler, error) {
	txHandler := &tCertTransactionHandlerImpl{}
	err := txHandler.init(handler)
	if err != nil {
		handler.client.error("Failed initiliazing transaction handler [%s]", err)

		return nil, err
	}

	return txHandler, nil
}

func (handler *tCertTransactionHandlerImpl) init(tCertHandler *tCertHandlerImpl) error {
	nonce, err := tCertHandler.client.createTransactionNonce()
	if err != nil {
		tCertHandler.client.error("Failed initiliazing transaction handler [%s]", err)

		return err
	}

	handler.tCertHandler = tCertHandler
	handler.nonce = nonce
	handler.binding = utils.Hash(append(handler.tCertHandler.tCert.GetCertificate().Raw, nonce...))

	return nil
}

// GetCertificateHandler returns the certificate handler relative to the certificate mapped to this transaction
func (handler *tCertTransactionHandlerImpl) GetCertificateHandler() (CertificateHandler, error) {
	return handler.tCertHandler, nil
}

// GetBinding returns an Binding to the underlying transaction layer
func (handler *tCertTransactionHandlerImpl) GetBinding() ([]byte, error) {
	return utils.Clone(handler.binding), nil
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (handler *tCertTransactionHandlerImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	return handler.tCertHandler.client.newChaincodeDeployUsingTCert(chaincodeDeploymentSpec, uuid, handler.tCertHandler.tCert, handler.nonce)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *tCertTransactionHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.tCertHandler.client.newChaincodeExecuteUsingTCert(chaincodeInvocation, uuid, handler.tCertHandler.tCert, handler.nonce)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *tCertTransactionHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.tCertHandler.client.newChaincodeQueryUsingTCert(chaincodeInvocation, uuid, handler.tCertHandler.tCert, handler.nonce)
}

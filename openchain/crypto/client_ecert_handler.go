package crypto

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type eCertHandlerImpl struct {
	client *clientImpl
}

type eCertTransactionHandlerImpl struct {
	client *clientImpl
}

func (handler *eCertHandlerImpl) init(client *clientImpl) error {
	handler.client = client

	return nil
}

// GetCertificate returns the TCert DER
func (handler *eCertHandlerImpl) GetCertificate() []byte {
	return utils.Clone(handler.client.node.enrollCert.Raw)
}

// Sign signs msg using the signing key corresponding to this TCert
func (handler *eCertHandlerImpl) Sign(msg []byte) ([]byte, error) {
	return handler.client.node.signWithEnrollmentKey(msg)
}

// Verify verifies msg using the verifying key corresponding to this TCert
func (handler *eCertHandlerImpl) Verify(signature []byte, msg []byte) error {
	ok, err := handler.client.node.verifyWithEnrollmentCert(signature, msg)
	if err != nil {
		return err
	}
	if !ok {
		return utils.ErrInvalidSignature
	}
	return nil
}

// GetTransactionHandler returns the transaction handler relative to this certificate
func (handler *eCertHandlerImpl) GetTransactionHandler() TransactionHandler {
	eCertTransactionHandlerImpl := &eCertTransactionHandlerImpl{}
	eCertTransactionHandlerImpl.init(handler.client)

	return eCertTransactionHandlerImpl
}

func (handler *eCertTransactionHandlerImpl) init(client *clientImpl) error {
	handler.client = client

	return nil
}

// GetCertificateHandler returns the certificate handler relative to the certificate mapped to this transaction
func (handler *eCertTransactionHandlerImpl) GetCertificateHandler() (CertificateHandler, error) {
	return handler.client.GetEnrollmentCertHandler()
}

// GetHook returns an Hook to the underlying transaction layer
func (handler *eCertTransactionHandlerImpl) GetHook() ([]byte, error) {
	return utils.Clone(handler.client.node.enrollCertHash), nil
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (handler *eCertTransactionHandlerImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeDeployUsingECert(chaincodeDeploymentSpec, uuid)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *eCertTransactionHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeExecuteUsingECert(chaincodeInvocation, uuid)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *eCertTransactionHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeQueryUsingECert(chaincodeInvocation, uuid)
}

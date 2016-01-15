package crypto

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type eCertHandlerImpl struct {
	client *clientImpl
}

func (handler *eCertHandlerImpl) init(client *clientImpl) error {
	handler.client = client

	return nil
}

// GetCertificate returns the TCert DER
func (handler *eCertHandlerImpl) GetCertificate() []byte {
	// TODO: clone this
	return handler.client.node.enrollCert.Raw
}

// GetHook returns an Hook to the underlying transaction layer
func (handler *eCertHandlerImpl) GetHook() ([]byte, error) {
	return nil, utils.ErrNotImplemented
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

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (handler *eCertHandlerImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeDeployUsingECert(chaincodeDeploymentSpec, uuid)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *eCertHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeExecuteUsingECert(chaincodeInvocation, uuid)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *eCertHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeQueryUsingECert(chaincodeInvocation, uuid)
}

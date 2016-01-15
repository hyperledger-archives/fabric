package crypto

import (
	"crypto/x509"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type tCertHandlerImpl struct {
	client *clientImpl

	tCertDER []byte
	tCert    *x509.Certificate
}

func (handler *tCertHandlerImpl) init(client *clientImpl, tCertDER []byte) error {
	handler.client = client

	// TODO: clone
	handler.tCertDER = tCertDER

	// Parse the DER
	tCert, err := utils.DERToX509Certificate(tCertDER)
	if err != nil {
		client.node.log.Error("Failed parsing TCert DER [%s].", err.Error())

		return err
	}
	handler.tCert = tCert

	return nil
}

// GetCertificate returns the TCert DER
func (handler *tCertHandlerImpl) GetCertificate() []byte {
	// TODO: clone this
	return handler.tCertDER
}

// GetHook returns an Hook to the underlying transaction layer
func (handler *tCertHandlerImpl) GetHook() ([]byte, error) {
	return nil, utils.ErrNotImplemented
}

// Sign signs msg using the signing key corresponding to this TCert
func (handler *tCertHandlerImpl) Sign(msg []byte) ([]byte, error) {
	return handler.client.signUsingTCertX509(handler.tCert, msg)
}

// Verify verifies msg using the verifying key corresponding to this TCert
func (handler *tCertHandlerImpl) Verify(signature []byte, msg []byte) error {
	return handler.client.verifyUsingTCertX509(handler.tCert, signature, msg)
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (handler *tCertHandlerImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeDeployUsingTCert(chaincodeDeploymentSpec, uuid, handler.tCertDER)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *tCertHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeExecuteUsingTCert(chaincodeInvocation, uuid, handler.tCertDER)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *tCertHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeQueryUsingTCert(chaincodeInvocation, uuid, handler.tCertDER)
}

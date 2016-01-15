package crypto

import (
	"crypto/x509"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type tCertHandlerImpl struct {
	client *clientImpl

	hook  []byte
	tCert *x509.Certificate
}

func (handler *tCertHandlerImpl) initDER(client *clientImpl, tCertDER []byte) error {
	handler.client = client

	// Parse the DER
	tCert, err := utils.DERToX509Certificate(tCertDER)
	if err != nil {
		client.node.log.Error("Failed parsing TCert DER [%s].", err.Error())

		return err
	}
	handler.tCert = tCert
	handler.hook = utils.Hash(tCert.Raw)

	return nil
}

func (handler *tCertHandlerImpl) initX509(client *clientImpl, tCert *x509.Certificate) error {
	handler.client = client
	handler.tCert = tCert
	handler.hook = utils.Hash(tCert.Raw)

	return nil
}

// GetCertificate returns the TCert DER
func (handler *tCertHandlerImpl) GetCertificate() []byte {
	return utils.Clone(handler.tCert.Raw)
}

// GetHook returns an Hook to the underlying transaction layer
func (handler *tCertHandlerImpl) GetHook() ([]byte, error) {
	return utils.Clone(handler.hook), nil
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
	return handler.client.newChaincodeDeployUsingTCert(chaincodeDeploymentSpec, uuid, handler.tCert.Raw)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *tCertHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeExecuteUsingTCert(chaincodeInvocation, uuid, handler.tCert.Raw)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *tCertHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeQueryUsingTCert(chaincodeInvocation, uuid, handler.tCert.Raw)
}

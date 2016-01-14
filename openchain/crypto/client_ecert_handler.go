package crypto

/*
import (
	"crypto/x509"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type eCertHandlerImpl struct {
	client *clientImpl

	tCertDER []byte
	tCert    *x509.Certificate
}

func (handler *eCertHandlerImpl) init(client *clientImpl) error {
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
func (handler *eCertHandlerImpl) GetCertificate() []byte {
	// TODO: clone this
	return handler.tCertDER
}

// Sign signs msg using the signing key corresponding to this TCert
func (handler *eCertHandlerImpl) Sign(msg []byte) ([]byte, error) {
	return handler.client.node.sign(hadler)signUsingTCertX509(handler.tCert, msg)
}

// Verify verifies msg using the verifying key corresponding to this TCert
func (handler *eCertHandlerImpl) Verify(signature []byte, msg []byte) error {
	return handler.client.verifyUsingTCertX509(handler.tCert, signature, msg)
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (handler *eCertHandlerImpl) NewChaincodeDeployTransaction(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeDeploy(chaincodeDeploymentSpec, uuid, handler.tCertDER)
}

// NewChaincodeExecute is used to execute chaincode's functions.
func (handler *eCertHandlerImpl) NewChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeExecute(chaincodeInvocation, uuid, handler.tCertDER)
}

// NewChaincodeQuery is used to query chaincode's functions.
func (handler *eCertHandlerImpl) NewChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	return handler.client.newChaincodeQuery(chaincodeInvocation, uuid, handler.tCertDER)
}
*/

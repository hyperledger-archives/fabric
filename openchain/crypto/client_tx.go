package crypto

import (
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (client *clientImpl) createTransactionNonce() ([]byte, error) {
	nonce, err := utils.GetRandomBytes(utils.NonceSize)
	if err != nil {
		client.error("Failed creating nonce [%s].", err.Error())
		return nil, err
	}

	return nonce, err
}

func (client *clientImpl) createDeployTx(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string, nonce []byte) (*obc.Transaction, error) {
	// Create a new transaction
	tx, err := obc.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, uuid)
	if err != nil {
		client.error("Failed creating new transaction [%s].", err.Error())
		return nil, err
	}

	// Copy metadata from ChaincodeSpec
	tx.Metadata = chaincodeDeploymentSpec.ChaincodeSpec.Metadata

	// Handle confidentiality
	if chaincodeDeploymentSpec.ChaincodeSpec.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// 1. set confidentiality level and nonce
		tx.ConfidentialityLevel = obc.ConfidentialityLevel_CONFIDENTIAL
		if nonce == nil {
			tx.Nonce, err = utils.GetRandomBytes(utils.NonceSize)
			if err != nil {
				client.error("Failed creating nonce [%s].", err.Error())
				return nil, err
			}
		} else {
			// TODO: check that it is a well formed nonce
			tx.Nonce = nonce
		}

		// 2. set confidentiality protocol version
		tx.ConfidentialityProtocolVersion = "1.2"

		// 3. encrypt tx
		err = client.encryptTx(tx)
		if err != nil {
			client.error("Failed encrypting payload [%s].", err.Error())
			return nil, err

		}
	}

	return tx, nil
}

func (client *clientImpl) createExecuteTx(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, nonce []byte) (*obc.Transaction, error) {
	/// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		client.error("Failed creating new transaction [%s].", err.Error())
		return nil, err
	}

	// Copy metadata from ChaincodeSpec
	tx.Metadata = chaincodeInvocation.ChaincodeSpec.Metadata

	// Handle confidentiality
	if chaincodeInvocation.ChaincodeSpec.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// 1. set confidentiality level and nonce
		tx.ConfidentialityLevel = obc.ConfidentialityLevel_CONFIDENTIAL
		if nonce == nil {
			tx.Nonce, err = utils.GetRandomBytes(utils.NonceSize)
			if err != nil {
				client.error("Failed creating nonce [%s].", err.Error())
				return nil, err
			}
		} else {
			// TODO: check that it is a well formed nonce
			tx.Nonce = nonce
		}

		// 2. set confidentiality protocol version
		tx.ConfidentialityProtocolVersion = "1.2"

		// 3. encrypt tx
		err = client.encryptTx(tx)
		if err != nil {
			client.error("Failed encrypting payload [%s].", err.Error())
			return nil, err

		}
	}

	return tx, nil
}

func (client *clientImpl) createQueryTx(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, nonce []byte) (*obc.Transaction, error) {
	// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		client.error("Failed creating new transaction [%s].", err.Error())
		return nil, err
	}

	// Copy metadata from ChaincodeSpec
	tx.Metadata = chaincodeInvocation.ChaincodeSpec.Metadata

	// Handle confidentiality
	if chaincodeInvocation.ChaincodeSpec.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// 1. set confidentiality level and nonce
		tx.ConfidentialityLevel = obc.ConfidentialityLevel_CONFIDENTIAL
		if nonce == nil {
			tx.Nonce, err = utils.GetRandomBytes(utils.NonceSize)
			if err != nil {
				client.error("Failed creating nonce [%s].", err.Error())
				return nil, err
			}
		} else {
			// TODO: check that it is a well formed nonce
			tx.Nonce = nonce
		}

		// 2. set confidentiality protocol version
		tx.ConfidentialityProtocolVersion = "1.2"

		// 3. encrypt tx
		err = client.encryptTx(tx)
		if err != nil {
			client.error("Failed encrypting payload [%s].", err.Error())
			return nil, err

		}
	}

	return tx, nil
}

func (client *clientImpl) newChaincodeDeployUsingTCert(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string, tCert tCert, nonce []byte) (*obc.Transaction, error) {
	// Create a new transaction
	tx, err := client.createDeployTx(chaincodeDeploymentSpec, uuid, nonce)
	if err != nil {
		client.error("Failed creating new deploy transaction [%s].", err.Error())
		return nil, err
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.debug("Appending certificate [% x].", tCert.GetCertificate().Raw)
	tx.Cert = tCert.GetCertificate().Raw

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := tCert.Sign(rawTx)
	if err != nil {
		client.error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.debug("Appending signature: [% x]", rawSignature)

	return tx, nil
}

func (client *clientImpl) newChaincodeExecuteUsingTCert(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, tCert tCert, nonce []byte) (*obc.Transaction, error) {
	/// Create a new transaction
	tx, err := client.createExecuteTx(chaincodeInvocation, uuid, nonce)
	if err != nil {
		client.error("Failed creating new execute transaction [%s].", err.Error())
		return nil, err
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.debug("Appending certificate [% x].", tCert.GetCertificate().Raw)
	tx.Cert = tCert.GetCertificate().Raw

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := tCert.Sign(rawTx)
	if err != nil {
		client.error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.debug("Appending signature [% x].", rawSignature)

	return tx, nil
}

func (client *clientImpl) newChaincodeQueryUsingTCert(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, tCert tCert, nonce []byte) (*obc.Transaction, error) {
	// Create a new transaction
	tx, err := client.createQueryTx(chaincodeInvocation, uuid, nonce)
	if err != nil {
		client.error("Failed creating new query transaction [%s].", err.Error())
		return nil, err
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.debug("Appending certificate [% x].", tCert.GetCertificate().Raw)
	tx.Cert = tCert.GetCertificate().Raw

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := tCert.Sign(rawTx)
	if err != nil {
		client.error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.debug("Appending signature [% x].", rawSignature)

	return tx, nil
}

func (client *clientImpl) newChaincodeDeployUsingECert(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string, nonce []byte) (*obc.Transaction, error) {
	// Create a new transaction
	tx, err := client.createDeployTx(chaincodeDeploymentSpec, uuid, nonce)
	if err != nil {
		client.error("Failed creating new deploy transaction [%s].", err.Error())
		return nil, err
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.debug("Appending certificate [% x].", client.enrollCert.Raw)
	tx.Cert = client.enrollCert.Raw

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := client.signWithEnrollmentKey(rawTx)
	if err != nil {
		client.error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.debug("Appending signature: [% x]", rawSignature)

	return tx, nil
}

func (client *clientImpl) newChaincodeExecuteUsingECert(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, nonce []byte) (*obc.Transaction, error) {
	/// Create a new transaction
	tx, err := client.createExecuteTx(chaincodeInvocation, uuid, nonce)
	if err != nil {
		client.error("Failed creating new execute transaction [%s].", err.Error())
		return nil, err
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.debug("Appending certificate [% x].", client.enrollCert.Raw)
	tx.Cert = client.enrollCert.Raw

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := client.signWithEnrollmentKey(rawTx)
	if err != nil {
		client.error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.debug("Appending signature [% x].", rawSignature)

	return tx, nil
}

func (client *clientImpl) newChaincodeQueryUsingECert(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, nonce []byte) (*obc.Transaction, error) {
	// Create a new transaction
	tx, err := client.createQueryTx(chaincodeInvocation, uuid, nonce)
	if err != nil {
		client.error("Failed creating new query transaction [%s].", err.Error())
		return nil, err
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.debug("Appending certificate [% x].", client.enrollCert.Raw)
	tx.Cert = client.enrollCert.Raw

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := client.signWithEnrollmentKey(rawTx)
	if err != nil {
		client.error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.debug("Appending signature [% x].", rawSignature)

	return tx, nil
}

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *clientImpl) checkTransaction(tx *obc.Transaction) error {
	if !client.isInitialized {
		return utils.ErrNotInitialized
	}

	if tx.Cert == nil && tx.Signature == nil {
		return utils.ErrTransactionMissingCert
	}

	if tx.Cert != nil && tx.Signature != nil {
		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			client.error("Failed unmarshalling cert [%s].", err.Error())
			return err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			client.error("Failed marshaling tx [%s].", err.Error())
			return err
		}
		tx.Signature = signature

		// 2. Verify signature
		ver, err := client.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			client.error("Failed marshaling tx [%s].", err.Error())
			return err
		}

		if ver {
			return nil
		}

		return utils.ErrInvalidTransactionSignature
	}

	return utils.ErrTransactionMissingCert
}

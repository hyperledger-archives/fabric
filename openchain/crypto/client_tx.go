package crypto

import (
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *clientImpl) newChaincodeDeploy(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string, rawTCert []byte) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, uuid)
	if err != nil {
		client.node.log.Error("Failed creating new transaction [%s].", err.Error())
		return nil, err
	}

	if chaincodeDeploymentSpec.ChaincodeSpec.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// 1. set confidentiality level and nonce
		tx.ConfidentialityLevel = obc.ConfidentialityLevel_CONFIDENTIAL
		tx.Nonce, err = utils.GetRandomBytes(utils.NonceSize)
		if err != nil {
			client.node.log.Error("Failed creating nonce [%s].", err.Error())
			return nil, err
		}

		// 2. encrypt tx
		err = client.encryptTx(tx)
		if err != nil {
			client.node.log.Error("Failed encrypting payload [%s].", err.Error())
			return nil, err

		}
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.node.log.Debug("Appending certificate [%s].", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.node.log.Error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.node.log.Debug("Signing tx [%s].", utils.EncodeBase64(rawTx))
	rawSignature, err := client.signUsingTCertDER(rawTCert, rawTx)
	if err != nil {
		client.node.log.Error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.node.log.Debug("Appending signature: [%s]", utils.EncodeBase64(rawSignature))

	return tx, nil
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *clientImpl) newChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, rawTCert []byte) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	/// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		client.node.log.Error("Failed creating new transaction [%s].", err.Error())
		return nil, err
	}

	if chaincodeInvocation.ChaincodeSpec.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// 1. set confidentiality level and nonce
		tx.ConfidentialityLevel = obc.ConfidentialityLevel_CONFIDENTIAL
		tx.Nonce, err = utils.GetRandomBytes(utils.NonceSize)
		if err != nil {
			client.node.log.Error("Failed creating nonce [%s].", err.Error())
			return nil, err
		}

		// 2. encrypt tx
		err = client.encryptTx(tx)
		if err != nil {
			client.node.log.Error("Failed encrypting payload [%s].", err.Error())
			return nil, err

		}
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.node.log.Debug("Appending certificate [%s].", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.node.log.Error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.node.log.Debug("Signing tx [%s].", utils.EncodeBase64(rawTx))
	rawSignature, err := client.signUsingTCertDER(rawTCert, rawTx)
	if err != nil {
		client.node.log.Error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.node.log.Debug("Appending signature [%s].", utils.EncodeBase64(rawSignature))

	return tx, nil
}

// NewChaincodeQuery is used to query chaincode's functions.
func (client *clientImpl) newChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string, rawTCert []byte) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	/// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		client.node.log.Error("Failed creating new transaction [%s].", err.Error())
		return nil, err
	}

	if chaincodeInvocation.ChaincodeSpec.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// 1. set confidentiality level and nonce
		tx.ConfidentialityLevel = obc.ConfidentialityLevel_CONFIDENTIAL
		tx.Nonce, err = utils.GetRandomBytes(utils.NonceSize)
		if err != nil {
			client.node.log.Error("Failed creating nonce [%s].", err.Error())
			return nil, err
		}

		// 2. encrypt tx
		err = client.encryptTx(tx)
		if err != nil {
			client.node.log.Error("Failed encrypting payload [%s].", err.Error())
			return nil, err

		}
	}

	// Sign the transaction

	// Append the certificate to the transaction
	client.node.log.Debug("Appending certificate [%s].", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.node.log.Error("Failed marshaling tx [%s].", err.Error())
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.node.log.Debug("Signing tx [%s].", utils.EncodeBase64(rawTx))
	rawSignature, err := client.signUsingTCertDER(rawTCert, rawTx)
	if err != nil {
		client.node.log.Error("Failed creating signature [%s].", err.Error())
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.node.log.Debug("Appending signature [%s].", utils.EncodeBase64(rawSignature))

	return tx, nil
}

package crypto

import (
	"crypto/rand"
	"encoding/asn1"
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/conf"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (client *clientImpl) encryptTx(tx *obc.Transaction) error {

	if len(tx.Nonce) == 0 {
		return errors.New("Failed encrypting payload. Invalid nonce.")
	}

	client.debug("confidentiality protocol version [%s]", tx.ConfidentialityProtocolVersion)
	switch tx.ConfidentialityProtocolVersion {
	case "1.1":
		client.debug("Using confidentiality protocol version 1.1")
		return client.encryptTxVersion1_1(tx)
	case "1.2":
		client.debug("Using confidentiality protocol version 1.2")
		return client.encryptTxVersion1_2(tx)
	}

	return utils.ErrInvalidProtocolVersion
}

func (client *clientImpl) encryptTxVersion1_1(tx *obc.Transaction) error {
	// client.enrollChainKey is an AES key represented as byte array
	enrollChainKey := client.enrollChainKey.([]byte)

	// Derive key
	txKey := utils.HMAC(enrollChainKey, tx.Nonce)

	//	client.log.Info("Deriving from :", utils.EncodeBase64(client.node.enrollChainKey))
	//	client.log.Info("Nonce  ", utils.EncodeBase64(tx.Nonce))
	//	client.log.Info("Derived key  ", utils.EncodeBase64(txKey))

	// Encrypt Payload
	payloadKey := utils.HMACTruncated(txKey, []byte{1}, utils.AESKeyLength)
	encryptedPayload, err := utils.CBCPKCS7Encrypt(payloadKey, tx.Payload)
	if err != nil {
		return err
	}
	tx.Payload = encryptedPayload

	// Encrypt ChaincodeID
	chaincodeIDKey := utils.HMACTruncated(txKey, []byte{2}, utils.AESKeyLength)
	encryptedChaincodeID, err := utils.CBCPKCS7Encrypt(chaincodeIDKey, tx.ChaincodeID)
	if err != nil {
		return err
	}
	tx.ChaincodeID = encryptedChaincodeID

	// Encrypt Metadata
	if len(tx.Metadata) != 0 {
		metadataKey := utils.HMACTruncated(txKey, []byte{3}, utils.AESKeyLength)
		encryptedMetadata, err := utils.CBCPKCS7Encrypt(metadataKey, tx.Metadata)
		if err != nil {
			return err
		}
		tx.Metadata = encryptedMetadata
	}

	client.debug("Encrypted ChaincodeID [% x].", tx.ChaincodeID)
	client.debug("Encrypted Payload [% x].", tx.Payload)
	client.debug("Encrypted Metadata [% x].", tx.Metadata)

	return nil
}

// chainCodeValidatorMessage1_2 represents a message to validators
type chainCodeValidatorMessage1_2 struct {
	PrivateKey []byte
	StateKey   []byte
}

func (client *clientImpl) encryptTxVersion1_2(tx *obc.Transaction) error {
	// Create (PK_C,SK_C) pair
	ccPrivateKey, err := client.eciesSPI.NewPrivateKey(rand.Reader, conf.GetDefaultCurve())
	if err != nil {
		client.error("Failed generate chaincode keypair: [%s]", err)

		return err
	}

	// Prepare message to the validators
	var (
		stateKey  []byte
		privBytes []byte
	)

	switch tx.Type {
	case obc.Transaction_CHAINCODE_NEW:
		// Prepare chaincode stateKey and privateKey
		stateKey, err = utils.GenAESKey()
		if err != nil {
			client.error("Failed creating state key: [%s]", err)

			return err
		}

		privBytes, err = client.eciesSPI.SerializePrivateKey(ccPrivateKey)
		if err != nil {
			client.error("Failed serializing chaincode key: [%s]", err)

			return err
		}

		break
	case obc.Transaction_CHAINCODE_QUERY:
		// Prepare chaincode stateKey and privateKey
		stateKey = utils.HMACTruncated(client.queryStateKey, append([]byte{6}, tx.Nonce...), utils.AESKeyLength)

		privBytes, err = client.eciesSPI.SerializePrivateKey(ccPrivateKey)
		if err != nil {
			client.error("Failed serializing chaincode key: [%s]", err)

			return err
		}

		break
	case obc.Transaction_CHAINCODE_EXECUTE:
		// Prepare chaincode stateKey and privateKey
		stateKey = make([]byte, 0)

		privBytes, err = client.eciesSPI.SerializePrivateKey(ccPrivateKey)
		if err != nil {
			client.error("Failed serializing chaincode key: [%s]", err)

			return err
		}
		break
	}

	// Encrypt message to the validators
	cipher, err := client.eciesSPI.NewAsymmetricCipherFromPublicKey(client.chainPublicKey)
	if err != nil {
		client.error("Failed creating new encryption scheme: [%s]", err)

		return err
	}

	msgToValidators, err := asn1.Marshal(chainCodeValidatorMessage1_2{privBytes, stateKey})
	if err != nil {
		client.error("Failed preparing message to the validators: [%s]", err)

		return err
	}

	encMsgToValidators, err := cipher.Process(msgToValidators)
	if err != nil {
		client.error("Failed encrypting message to the validators: [%s]", err)

		return err
	}
	// TODO: change name to Key
	tx.ToValidators = encMsgToValidators

	client.debug("Message to Validator: [% x]", tx.ToValidators)

	// Encrypt the rest of the fields

	// Init with chainccode pk
	cipher, err = client.eciesSPI.NewAsymmetricCipherFromPublicKey(ccPrivateKey.GetPublicKey())
	if err != nil {
		client.error("Failed initiliazing encryption scheme: [%s]", err)

		return err
	}

	// Encrypt chaincodeID using pkC
	encryptedChaincodeID, err := cipher.Process(tx.ChaincodeID)
	if err != nil {
		client.error("Failed encrypting chaincodeID: [%s]", err)

		return err
	}
	tx.ChaincodeID = encryptedChaincodeID

	// Encrypt payload using pkC
	encryptedPayload, err := cipher.Process(tx.Payload)
	if err != nil {
		client.error("Failed encrypting payload: [%s]", err)

		return err
	}
	tx.Payload = encryptedPayload

	// Encrypt metadata using pkC
	if len(tx.Metadata) != 0 {
		encryptedMetadata, err := cipher.Process(tx.Metadata)
		if err != nil {
			client.error("Failed encrypting metadata: [%s]", err)

			return err
		}
		tx.Metadata = encryptedMetadata
	}

	return nil
}

package crypto

import (
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (client *clientImpl) encryptTx(tx *obc.Transaction) error {

	if len(tx.Nonce) == 0 {
		return errors.New("Failed encrypting payload. Invalid nonce.")
	}

	// Derive key
	txKey := utils.HMAC(client.node.enrollChainKey, tx.Nonce)

	//	client.node.info("Deriving from :", utils.EncodeBase64(client.node.enrollChainKey))
	//	client.node.info("Nonce  ", utils.EncodeBase64(tx.Nonce))
	//	client.node.info("Derived key  ", utils.EncodeBase64(txKey))

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

	client.node.debug("Encrypted ChaincodeID [% x].", tx.ChaincodeID)
	client.node.debug("Encrypted Payload [% x].", tx.Payload)
	client.node.debug("Encrypted Metadata [% x].", tx.Metadata)

	return nil
}

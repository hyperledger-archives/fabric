package crypto

import (
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (client *clientImpl) encryptTx(tx *obc.Transaction) error {

	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return errors.New("Failed encrypting payload. Invalid nonce.")
	}

	// Derive key
	txKey := utils.HMAC(client.node.enrollChainKey, tx.Nonce)

	//	client.node.log.Info("Deriving from :", utils.EncodeBase64(client.node.enrollChainKey))
	//	client.node.log.Info("Nonce  ", utils.EncodeBase64(tx.Nonce))
	//	client.node.log.Info("Derived key  ", utils.EncodeBase64(txKey))

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

	client.node.log.Debug("Encrypted Payload [%s].", utils.EncodeBase64(tx.Payload))
	client.node.log.Debug("Encrypted ChaincodeID [%s].", utils.EncodeBase64(tx.ChaincodeID))

	return nil
}

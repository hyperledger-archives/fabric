package crypto

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
	)

func (client *clientImpl) encryptTx(tx *obc.Transaction) error {

	// Derive key
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return errors.New("Failed encrypting payload. Invalid nonce.")
	}
	txKey := utils.HMAC(client.node.enrollChainKey, tx.Nonce)

	log.Info("Deriving from %s", utils.EncodeBase64(client.node.enrollChainKey))
	log.Info("Nonce %s", utils.EncodeBase64(tx.Nonce))
	log.Info("Derived key %s", utils.EncodeBase64(txKey))

	// Encrypt using the derived key
	payloadKey := utils.HMACTruncated(txKey, []byte{1}, utils.AESKeyLength)
	encryptedPayload, err := utils.CBCPKCS7Encrypt(payloadKey, tx.Payload)
	if err != nil {
		return err
	}
	tx.EncryptedPayload = encryptedPayload
	tx.Payload = nil

	chaincodeIdKey := utils.HMACTruncated(txKey, []byte{2}, utils.AESKeyLength)
	rawChaincodeId, err := proto.Marshal(tx.ChaincodeID)
	if err != nil {
		return err
	}
	tx.EncryptedChaincodeID, err = utils.CBCPKCS7Encrypt(chaincodeIdKey, rawChaincodeId)
	if err != nil {
		return err
	}
	tx.ChaincodeID = nil

	log.Info("Encrypted Payload %s", utils.EncodeBase64(tx.EncryptedPayload))
	log.Info("Encrypted ChaincodeID %s", utils.EncodeBase64(tx.EncryptedChaincodeID))

	return nil
}


package client

import (
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (client *Client) encryptPayload(tx *obc.Transaction) ([]byte, error) {

	// Derive key
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Failed encrypting payload. Invalid nonce.")
	}
	key := utils.HMACTruncated(client.enrollChainKey, tx.Nonce, utils.AES_KEY_LENGTH_BYTES)
	//	log.Info("Deriving from %s", utils.EncodeBase64(client.enrollChainKey))
	//	log.Info("Nonce %s", utils.EncodeBase64(tx.Nonce))
	//	log.Info("Derived key %s", utils.EncodeBase64(key))
	//
	// Encrypt using the derived key
	return utils.CBCPKCS7Encrypt(key, tx.Payload)
}

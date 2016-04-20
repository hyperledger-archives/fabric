package crypto

import (
	"github.com/hyperledger/fabric/core/crypto/primitives"
	obc "github.com/hyperledger/fabric/protos"
)

type clientConfidentialityProcessorV1_1 struct {
	client *clientImpl
}

func (cp *clientConfidentialityProcessorV1_1) getVersion() string {
	return "1.1"
}

func (cp *clientConfidentialityProcessorV1_1) process(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()

	// client.enrollChainKey is an AES key represented as byte array
	enrollChainKey := cp.client.enrollSymChainKey

	// Derive key
	txKey := primitives.HMAC(enrollChainKey, tx.Nonce)

	//	client.log.Info("Deriving from :", utils.EncodeBase64(client.node.enrollChainKey))
	//	client.log.Info("Nonce  ", utils.EncodeBase64(tx.Nonce))
	//	client.log.Info("Derived key  ", utils.EncodeBase64(txKey))

	// Encrypt Payload
	payloadKey := primitives.HMACTruncated(txKey, []byte{1}, primitives.AESKeyLength)
	encryptedPayload, err := primitives.CBCPKCS7Encrypt(payloadKey, tx.Payload)
	if err != nil {
		return nil, err
	}
	tx.Payload = encryptedPayload

	// Encrypt ChaincodeID
	chaincodeIDKey := primitives.HMACTruncated(txKey, []byte{2}, primitives.AESKeyLength)
	encryptedChaincodeID, err := primitives.CBCPKCS7Encrypt(chaincodeIDKey, tx.ChaincodeID)
	if err != nil {
		return nil, err
	}
	tx.ChaincodeID = encryptedChaincodeID

	// Encrypt Metadata
	if len(tx.Metadata) != 0 {
		metadataKey := primitives.HMACTruncated(txKey, []byte{3}, primitives.AESKeyLength)
		encryptedMetadata, err := primitives.CBCPKCS7Encrypt(metadataKey, tx.Metadata)
		if err != nil {
			return nil, err
		}
		tx.Metadata = encryptedMetadata
	}

	return tx, nil
}

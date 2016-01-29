package crypto

import (
	"crypto/rand"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

var (
	chainPrivateKey ecies.PrivateKey
	chainPublicKey  ecies.PublicKey
)

func init() {
	var err error
	chainPrivateKey, err = generic.NewPrivateKey(rand.Reader, utils.DefaultCurve)
	if err != nil {
		panic(err)
	}

	chainPublicKey = chainPrivateKey.GetPublicKey()
}

func (client *clientImpl) encryptTxVersion1_2(tx *obc.Transaction) error {
	// Create (PK_C,SK_C) pair
	priv, err := generic.NewPrivateKey(rand.Reader, utils.DefaultCurve)
	if err != nil {
		client.node.log.Error("Failed generate chaincode keypair: [%s]", err)

		return err
	}

	// Encrypt priv using chainPublicKey
	es, err := generic.NewEncryptionSchemeFromPublicKey(chainPublicKey)
	if err != nil {
		client.node.log.Error("Failed creating new encryption scheme: [%s]", err)

		return err
	}

	privBytes, err := generic.SerializePrivateKey(priv)
	if err != nil {
		client.node.log.Error("Failed serializing chaincode key: [%s]", err)

		return err
	}

	encryptedSKC, err := es.Process(privBytes)
	if err != nil {
		client.node.log.Error("Failed encrypting chaincodeID: [%s]", err)

		return err
	}
	tx.Key = encryptedSKC

	// Init with chainccode pk
	err = es.Init(priv.GetPublicKey())
	if err != nil {
		client.node.log.Error("Failed initiliazing encryption scheme: [%s]", err)

		return err
	}

	// Encrypt chaincodeID using pkC
	encryptedChaincodeID, err := es.Process(tx.ChaincodeID)
	if err != nil {
		client.node.log.Error("Failed encrypting chaincodeID: [%s]", err)

		return err
	}
	tx.ChaincodeID = encryptedChaincodeID

	// Encrypt payload using pkC
	encryptedPayload, err := es.Process(tx.Payload)
	if err != nil {
		client.node.log.Error("Failed encrypting payload: [%s]", err)

		return err
	}
	tx.Payload = encryptedPayload

	// Encrypt metadata using pkC
	if len(tx.Metadata) != 0 {
		encryptedMetadata, err := es.Process(tx.Metadata)
		if err != nil {
			client.node.log.Error("Failed encrypting metadata: [%s]", err)

			return err
		}
		tx.Metadata = encryptedMetadata
	}

	return nil
}

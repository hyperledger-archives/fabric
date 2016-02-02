package crypto

import (
	"crypto/rand"
	"encoding/asn1"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

// chainCodeValidatorMessage1_2 represents a message to validators
type chainCodeValidatorMessage1_2 struct {
	PrivateKey []byte
	StateKey   []byte
}

func (client *clientImpl) encryptTxVersion1_2(tx *obc.Transaction) error {
	// Create (PK_C,SK_C) pair
	ccPrivateKey, err := generic.NewPrivateKey(rand.Reader, utils.DefaultCurve)
	if err != nil {
		client.log.Error("Failed generate chaincode keypair: [%s]", err)

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
			client.log.Error("Failed creating state key: [%s]", err)

			return err
		}

		privBytes, err = generic.SerializePrivateKey(ccPrivateKey)
		if err != nil {
			client.log.Error("Failed serializing chaincode key: [%s]", err)

			return err
		}

		break
	case obc.Transaction_CHAINCODE_QUERY:
		// Prepare chaincode stateKey and privateKey
		stateKey = utils.HMACTruncated(client.queryStateKey, append([]byte{6}, tx.Nonce...), utils.AESKeyLength)

		privBytes, err = generic.SerializePrivateKey(ccPrivateKey)
		if err != nil {
			client.log.Error("Failed serializing chaincode key: [%s]", err)

			return err
		}

		break
	case obc.Transaction_CHAINCODE_EXECUTE:
		// Prepare chaincode stateKey and privateKey
		stateKey = make([]byte, 0)

		privBytes, err = generic.SerializePrivateKey(ccPrivateKey)
		if err != nil {
			client.log.Error("Failed serializing chaincode key: [%s]", err)

			return err
		}
		break
	}

	// Encrypt message to the validators
	es, err := generic.NewEncryptionSchemeFromPublicKey(client.chainPublicKey)
	if err != nil {
		client.log.Error("Failed creating new encryption scheme: [%s]", err)

		return err
	}

	msgToValidators, err := asn1.Marshal(chainCodeValidatorMessage1_2{privBytes, stateKey})
	if err != nil {
		client.log.Error("Failed preparing message to the validators: [%s]", err)

		return err
	}

	encryptedSKC, err := es.Process(msgToValidators)
	if err != nil {
		client.log.Error("Failed encrypting message to the validators: [%s]", err)

		return err
	}
	tx.Key = encryptedSKC

	// Encrypt the rest of the fields

	// Init with chainccode pk
	es, err = generic.NewEncryptionSchemeFromPublicKey(ccPrivateKey.GetPublicKey())
	if err != nil {
		client.log.Error("Failed initiliazing encryption scheme: [%s]", err)

		return err
	}

	// Encrypt chaincodeID using pkC
	encryptedChaincodeID, err := es.Process(tx.ChaincodeID)
	if err != nil {
		client.log.Error("Failed encrypting chaincodeID: [%s]", err)

		return err
	}
	tx.ChaincodeID = encryptedChaincodeID

	// Encrypt payload using pkC
	encryptedPayload, err := es.Process(tx.Payload)
	if err != nil {
		client.log.Error("Failed encrypting payload: [%s]", err)

		return err
	}
	tx.Payload = encryptedPayload

	// Encrypt metadata using pkC
	if len(tx.Metadata) != 0 {
		encryptedMetadata, err := es.Process(tx.Metadata)
		if err != nil {
			client.log.Error("Failed encrypting metadata: [%s]", err)

			return err
		}
		tx.Metadata = encryptedMetadata
	}

	return nil
}

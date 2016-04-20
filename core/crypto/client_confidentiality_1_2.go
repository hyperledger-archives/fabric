package crypto

import (
	"crypto/rand"
	"encoding/asn1"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	obc "github.com/hyperledger/fabric/protos"
)

type clientConfidentialityProcessorV1_2 struct {
	client *clientImpl
}

// chainCodeValidatorMessage1_2 represents a message to validators
type messageToValidatorsV1_2 struct {
	PrivateKey []byte
	StateKey   []byte
}

func (cp *clientConfidentialityProcessorV1_2) getVersion() string {
	return "1.2"
}

func (cp *clientConfidentialityProcessorV1_2) process(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()

	// Create (PK_C,SK_C) pair
	skC, err := cp.client.acSPI.NewDefaultPrivateKey(rand.Reader)
	if err != nil {
		cp.client.error("Failed generate chaincode keypair: [%s]", err)

		return nil, err
	}

	// Prepare keys
	var (
		kState   []byte
		skCBytes []byte
	)

	switch tx.Type {
	case obc.Transaction_CHAINCODE_DEPLOY:
		// Prepare chaincode stateKey and privateKey
		kState, err = primitives.GenAESKey()
		if err != nil {
			cp.client.error("Failed creating state key: [%s]", err)

			return nil, err
		}

		skCBytes, err = cp.client.acSPI.SerializePrivateKey(skC)
		if err != nil {
			cp.client.error("Failed serializing chaincode key: [%s]", err)

			return nil, err
		}

		break
	case obc.Transaction_CHAINCODE_QUERY:
		// Prepare chaincode stateKey and privateKey
		kState = primitives.HMACTruncated(cp.client.queryStateKey, append([]byte{6}, tx.Nonce...), primitives.AESKeyLength)

		skCBytes, err = cp.client.acSPI.SerializePrivateKey(skC)
		if err != nil {
			cp.client.error("Failed serializing chaincode key: [%s]", err)

			return nil, err
		}

		break
	case obc.Transaction_CHAINCODE_INVOKE:
		// Prepare chaincode stateKey and privateKey
		kState = make([]byte, 0)

		skCBytes, err = cp.client.acSPI.SerializePrivateKey(skC)
		if err != nil {
			cp.client.error("Failed serializing chaincode key: [%s]", err)

			return nil, err
		}
		break
	}

	// Encrypt message to the validators
	cipher, err := cp.client.acSPI.NewAsymmetricCipherFromPublicKey(cp.client.chainPublicKey)
	if err != nil {
		cp.client.error("Failed creating new encryption scheme: [%s]", err)

		return nil, err
	}

	msgToValidators, err := asn1.Marshal(messageToValidatorsV1_2{skCBytes, kState})
	if err != nil {
		cp.client.error("Failed preparing message to the validators: [%s]", err)

		return nil, err
	}

	ctMsgToValidators, err := cipher.Process(msgToValidators)
	if err != nil {
		cp.client.error("Failed encrypting message to the validators: [%s]", err)

		return nil, err
	}
	tx.ToValidators = ctMsgToValidators

	cp.client.debug("Message to Validator: [% x]", tx.ToValidators)

	// Encrypt the rest of the fields (ChaincodeID, Payload, metadata) using pkC

	// Init with pkC
	cipher, err = cp.client.acSPI.NewAsymmetricCipherFromPublicKey(skC.GetPublicKey())
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}

	// Encrypt chaincodeID using pkC
	ctChaincodeID, err := cipher.Process(tx.ChaincodeID)
	if err != nil {
		cp.client.error("Failed encrypting chaincodeID: [%s]", err)

		return nil, err
	}
	tx.ChaincodeID = ctChaincodeID

	// Encrypt payload using pkC
	ctPayload, err := cipher.Process(tx.Payload)
	if err != nil {
		cp.client.error("Failed encrypting payload: [%s]", err)

		return nil, err
	}
	tx.Payload = ctPayload

	// Encrypt metadata using pkC
	if len(tx.Metadata) != 0 {
		ctMetadata, err := cipher.Process(tx.Metadata)
		if err != nil {
			cp.client.error("Failed encrypting metadata: [%s]", err)

			return nil, err
		}
		tx.Metadata = ctMetadata
	}

	return tx, nil
}

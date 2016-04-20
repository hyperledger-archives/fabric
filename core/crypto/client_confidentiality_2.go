package crypto

import (
	"crypto/rand"
	"encoding/asn1"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

type clientConfidentialityProcessorV2 struct {
	client *clientImpl

	scSPI primitives.StreamCipherSPI
	acSPI primitives.AsymmetricCipherSPI
}

func (cp *clientConfidentialityProcessorV2) getVersion() string {
	return "2.0"
}

func (cp *clientConfidentialityProcessorV2) process(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()
	switch tx.Type {
	case obc.Transaction_CHAINCODE_DEPLOY:
		return cp.processDeploy(ctx)
	case obc.Transaction_CHAINCODE_INVOKE:
		return cp.processExecute(ctx)
	case obc.Transaction_CHAINCODE_QUERY:
		return cp.processQuery(ctx)
	}

	return nil, utils.ErrInvalidTransactionType
}

func (cp *clientConfidentialityProcessorV2) processDeploy(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()
	chaincodeSpec := ctx.GetChaincodeSpec()

	// Create (PK_C,SK_C) pair
	skC, err := cp.acSPI.NewDefaultPrivateKey(rand.Reader)
	if err != nil {
		cp.client.error("Failed generate chaincode keypair: [%s]", err)

		return nil, err
	}

	// Create kHeader, kCode, kState
	kHeader, kHeaderBytes, err := cp.scSPI.GenerateKeyAndSerialize()
	if err != nil {
		cp.client.error("Failed creating header key: [%s]", err)

		return nil, err
	}

	kCode, kCodeBytes, err := cp.scSPI.GenerateKeyAndSerialize()
	if err != nil {
		cp.client.error("Failed creating code key: [%s]", err)

		return nil, err
	}

	_, kStateBytes, err := cp.scSPI.GenerateKeyAndSerialize()
	if err != nil {
		cp.client.error("Failed creating state key: [%s]", err)

		return nil, err
	}

	skCBytes, err := cp.acSPI.SerializePrivateKey(skC)
	if err != nil {
		cp.client.error("Failed serializing chaincode key: [%s]", err)

		return nil, err
	}

	pkCBytes, err := cp.acSPI.SerializePublicKey(skC.GetPublicKey())
	if err != nil {
		cp.client.error("Failed serializing chaincode key: [%s]", err)

		return nil, err
	}

	// Encrypt message to the validators

	messageToValidatorsChain, err := asn1.Marshal(deployValidatorsMessageChainV2{"skC", skCBytes})
	if err != nil {
		cp.client.error("Failed marshalling message to the validators (chain): [%s]", err)

		return nil, err
	}
	messageToValidatorsChaincode, err := asn1.Marshal(deployValidatorsMessageChaincodeV2{
		"header", kHeaderBytes, "code", kCodeBytes, "state", kStateBytes,
	})
	if err != nil {
		cp.client.error("Failed marshalling message to the validators (chaincode): [%s]", err)

		return nil, err
	}

	aCipher, err := cp.acSPI.NewAsymmetricCipherFromPublicKey(cp.client.chainPublicKey)
	if err != nil {
		cp.client.error("Failed creating new encryption scheme: [%s]", err)

		return nil, err
	}
	ctMessageToValidatorsChain, err := aCipher.Process(messageToValidatorsChain)
	if err != nil {
		cp.client.error("Failed encrypting message to the validators (chain): [%s]", err)

		return nil, err
	}

	aCipher, err = cp.acSPI.NewAsymmetricCipherFromPublicKey(skC.GetPublicKey())
	if err != nil {
		cp.client.error("Failed creating new encryption scheme: [%s]", err)

		return nil, err
	}
	ctMessageToValidatorsChaincode, err := aCipher.Process(messageToValidatorsChaincode)
	if err != nil {
		cp.client.error("Failed encrypting message to the validators (chaincode): [%s]", err)

		return nil, err
	}

	messageToValidators, err := asn1.Marshal(deployValidatorsMessageV2{"depValMes", ctMessageToValidatorsChain, ctMessageToValidatorsChaincode})
	if err != nil {
		cp.client.error("Failed marshalling message to the validators: [%s]", err)

		return nil, err
	}
	tx.ToValidators = messageToValidators

	cp.client.debug("Message to Validator: [% x]", tx.ToValidators)

	// Encrypt message to the users
	if len(chaincodeSpec.Users) > 0 {
		// Messages to users
		userMessages := make([]deployUserMessageV2, len(chaincodeSpec.Users))
		for i, chaincodeUserSpec := range chaincodeSpec.Users {

			// Prepare keys
			keysToUser := deployuserKeysV2{}
			keysToUser.PkC = pkCBytes
			keysToUser.PkCFlag = "pkC"
			keysToUser.KHeader = kHeaderBytes
			keysToUser.KHeaderFlag = "header"
			keysToUser.KStateFlag = "state"
			keysToUser.KCodeFlag = "code"

			if chaincodeUserSpec.GrantCodeAccess {
				keysToUser.KCode = kCodeBytes
			}

			if chaincodeUserSpec.GrantStateAccess {
				keysToUser.KState = kStateBytes
			}

			keysToUserRaw, err := asn1.Marshal(keysToUser)
			if err != nil {
				cp.client.error("Failed marshalling user message: [%s]", err)

				return nil, err
			}

			cp.client.debug("chaincodeUserSpec.PublicKey: [% x]", chaincodeUserSpec.PublicKey)

			aCipher, err := cp.acSPI.NewAsymmetricCipherFromSerializedPublicKey(chaincodeUserSpec.PublicKey)
			if err != nil {
				cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

				return nil, err
			}
			ctKeysToUser, err := aCipher.Process(keysToUserRaw)
			if err != nil {
				cp.client.error("Failed encrypting chaincodeID: [%s]", err)

				return nil, err
			}

			// Prepare user message
			userMessages[i] = deployUserMessageV2{chaincodeUserSpec.Cert, ctKeysToUser}
		}

		// Message to creator
		creatorMessage := deployCreatorMessageV2{"skC", skCBytes}
		creatorMessageRaw, err := asn1.Marshal(creatorMessage)
		if err != nil {
			cp.client.error("Failed marshalling creator message: [%s]", err)

			return nil, err
		}
		aCipher, err := cp.acSPI.NewAsymmetricCipherFromPublicKey(cp.client.enrollEncryptionKey.GetPublicKey())
		if err != nil {
			cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

			return nil, err
		}
		ctCreatorMessage, err := aCipher.Process(creatorMessageRaw)
		if err != nil {
			cp.client.error("Failed encrypting chaincodeID: [%s]", err)

			return nil, err
		}

		// Marshall everything
		toUsers, err := asn1.Marshal(deployUserMessagesV2{userMessages, ctCreatorMessage})
		if err != nil {
			cp.client.error("Failed marshalling message to the users: [%s]", err)

			return nil, err
		}

		tx.ToUsers = toUsers
	} else {
		// Message to creator
		creatorMessage := deployCreatorMessageV2{"skC", skCBytes}
		creatorMessageRaw, err := asn1.Marshal(creatorMessage)
		if err != nil {
			cp.client.error("Failed marshalling creator message: [%s]", err)

			return nil, err
		}
		aCipher, err := cp.acSPI.NewAsymmetricCipherFromPublicKey(cp.client.enrollEncryptionKey.GetPublicKey())
		if err != nil {
			cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

			return nil, err
		}
		ctCreatorMessage, err := aCipher.Process(creatorMessageRaw)
		if err != nil {
			cp.client.error("Failed encrypting chaincodeID: [%s]", err)

			return nil, err
		}

		// Marshall everything
		toUsers, err := asn1.Marshal(deployUserMessagesV2{nil, ctCreatorMessage})
		if err != nil {
			cp.client.error("Failed marshalling message to the users: [%s]", err)

			return nil, err
		}

		tx.ToUsers = toUsers
	}

	// Encrypt the rest of the fields (ChaincodeID, Payload, metadata)

	// Encrypt chaincodeID using kHeader
	cp.client.debug("CHAINCODE ID: [% x]", tx.ChaincodeID)
	headerMessage, err := asn1.Marshal(headerMessageV2{"chaincodeID", tx.ChaincodeID})
	if err != nil {
		cp.client.error("Failed marshalling message to the validators (chain): [%s]", err)

		return nil, err
	}

	sCipher, err := cp.scSPI.NewStreamCipherForEncryptionFromKey(kHeader)
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}

	ctChaincodeID, err := sCipher.Process(headerMessage)
	if err != nil {
		cp.client.error("Failed encrypting chaincodeID: [%s]", err)

		return nil, err
	}
	tx.ChaincodeID = ctChaincodeID

	// Encrypt payload using kCode
	sCipher, err = cp.scSPI.NewStreamCipherForEncryptionFromKey(kCode)
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}
	ctPayload, err := sCipher.Process(tx.Payload)
	if err != nil {
		cp.client.error("Failed encrypting payload: [%s]", err)

		return nil, err
	}
	tx.Payload = ctPayload

	// Encrypt metadata using kCode
	if len(tx.Metadata) != 0 {
		ctMetadata, err := sCipher.Process(tx.Metadata)
		if err != nil {
			cp.client.error("Failed encrypting metadata: [%s]", err)

			return nil, err
		}
		tx.Metadata = ctMetadata
	}

	return tx, nil
}

func (cp *clientConfidentialityProcessorV2) processExecute(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()
	chaincodeSpec := ctx.GetChaincodeSpec()

	pkC, deployCertHandler, err := cp.getPKCFromDeployTransaction(chaincodeSpec.Parent)
	if err != nil {
		cp.client.error("Failed getting pkC from deploy transaction: [%s]", err)

		return nil, err
	}

	// Create kI
	kI, kIBytes, err := cp.scSPI.GenerateKeyAndSerialize()

	// To the validators

	validatorMessages := eValidatorMessagesV2{"kI", kIBytes}
	validatorMessagesRaw, err := asn1.Marshal(validatorMessages)
	if err != nil {
		cp.client.error("Failed marshalling creator message: [%s]", err)

		return nil, err
	}
	aCipher, err := cp.acSPI.NewAsymmetricCipherFromPublicKey(pkC)
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}
	validatorMessagesCT, err := aCipher.Process(validatorMessagesRaw)
	if err != nil {
		cp.client.error("Failed encrypting chaincodeID: [%s]", err)

		return nil, err
	}
	tx.ToValidators = validatorMessagesCT

	// To the users (only the invoker)

	invokerMessage := eInvokerMessageV2{"kI", kIBytes}
	invokerMessageRaw, err := asn1.Marshal(invokerMessage)
	if err != nil {
		cp.client.error("Failed marshalling creator message: [%s]", err)

		return nil, err
	}
	aCipher, err = cp.acSPI.NewAsymmetricCipherFromPublicKey(cp.client.enrollEncryptionKey.GetPublicKey())
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}
	invokerMessageCT, err := aCipher.Process(invokerMessageRaw)
	if err != nil {
		cp.client.error("Failed encrypting chaincodeID: [%s]", err)

		return nil, err
	}

	// Marshall everything
	toUsers, err := asn1.Marshal(eUserMessagesV2{invokerMessageCT})
	if err != nil {
		cp.client.error("Failed marshalling message to the users: [%s]", err)

		return nil, err
	}

	tx.ToUsers = toUsers

	// Encrypt the rest of the fields (ChaincodeID, Payload, metadata)

	// Encrypt chaincodeID using PkChain
	headerMessage, err := asn1.Marshal(headerMessageV2{"chaincodeID", tx.ChaincodeID})
	if err != nil {
		cp.client.error("Failed marshalling message to the validators (chain): [%s]", err)

		return nil, err
	}

	aCipher, err = cp.acSPI.NewAsymmetricCipherFromPublicKey(cp.client.chainPublicKey)
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}

	ctChaincodeID, err := aCipher.Process(headerMessage)
	if err != nil {
		cp.client.error("Failed encrypting chaincodeID: [%s]", err)

		return nil, err
	}
	tx.ChaincodeID = ctChaincodeID

	// Encrypt metadata using kI
	sCipher, err := cp.scSPI.NewStreamCipherForEncryptionFromKey(kI)
	if err != nil {
		cp.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}

	if len(tx.Metadata) != 0 {
		ctMetadata, err := sCipher.Process(tx.Metadata)
		if err != nil {
			cp.client.error("Failed encrypting metadata: [%s]", err)

			return nil, err
		}
		tx.Metadata = ctMetadata
	}

	// Encrypt payload using kI. Sign tx.Payload||tx.Bindings
	payloadSigma, err := deployCertHandler.Sign(append(tx.Payload, ctx.GetBinding()...))
	if err != nil {
		cp.client.error("Failed signing payload: [%s]", err)

		return nil, err
	}

	payload, err := asn1.Marshal(ePayloadV2{"payload", tx.Payload, primitives.Hash(tx.Cert), payloadSigma})
	if err != nil {
		cp.client.error("Failed marshalling message to the validators (chain): [%s]", err)

		return nil, err
	}

	payloadCT, err := sCipher.Process(payload)
	if err != nil {
		cp.client.error("Failed encrypting payload: [%s]", err)

		return nil, err
	}
	tx.Payload = payloadCT

	return tx, nil
}

func (cp *clientConfidentialityProcessorV2) processQuery(ctx TransactionContext) (*obc.Transaction, error) {
	return cp.processExecute(ctx)
}

func (cp *clientConfidentialityProcessorV2) getPKCFromDeployTransaction(tx *obc.Transaction) (primitives.PublicKey, CertificateHandler, error) {
	if tx == nil {
		return nil, nil, utils.ErrNilArgument
	}

	// Retrieve pkC from the current user

	// TODO: check the rest for all unmarshal
	userMessages := new(deployUserMessagesV2)
	_, err := asn1.Unmarshal(tx.ToUsers, userMessages)
	if err != nil {
		cp.client.error("Failed unmarshalling message to users [%s].", err.Error())
		return nil, nil, err
	}
	aCipher, err := cp.acSPI.NewAsymmetricCipherFromPrivateKey(cp.client.enrollEncryptionKey)
	if err != nil {
		cp.client.error("Failed creating asymmetric cipher [%s].", err.Error())
		return nil, nil, err
	}

	cp.client.error("Checking user messages [%d].", len(userMessages.UserMessages))
	for _, userMessage := range userMessages.UserMessages {
		cp.client.error("User messages [% x].", userMessage.Cert)

		certHandler, err := cp.client.GetTCertificateHandlerFromDER(userMessage.Cert)
		if err != nil {
			cp.client.error("Failed decripting [%s].", err.Error())
			continue
		}

		// TODO: Decrypt keys
		userKeysRaw, err := aCipher.Process(userMessage.Keys)
		if err != nil {
			cp.client.error("Failed decripting [%s].", err.Error())
			continue
		}

		userKeys := new(deployuserKeysV2)
		_, err = asn1.Unmarshal(userKeysRaw, userKeys)
		if err != nil {
			cp.client.error("Failed unmarshalling message to user [%s].", err.Error())
			return nil, nil, err
		}

		// Check flags
		if userKeys.PkCFlag != "pkC" {
			cp.client.error("Invalid pkC flag.")
			continue
		}
		if userKeys.KCodeFlag != "code" {
			cp.client.error("Invalid code flag.")
			continue
		}
		if userKeys.KHeaderFlag != "header" {
			cp.client.error("Invalid header flag.")
			continue
		}
		if userKeys.KStateFlag != "state" {
			cp.client.error("Invalid state flag.")
			continue
		}

		pkC, err := cp.acSPI.DeserializePublicKey(userKeys.PkC)
		if err != nil {
			cp.client.error("Failed deserializing chaincode public key [%s].", err.Error())
			return nil, nil, err
		}

		return pkC, certHandler, nil
	}

	// Try creator

	deployCreatorMessageRaw, err := aCipher.Process(userMessages.CreatorMessage)
	if err != nil {
		cp.client.error("Failed decripting [%s].", err.Error())
		return nil, nil, utils.ErrInvalidReference
	}

	deployCreatorMessage := new(deployCreatorMessageV2)
	_, err = asn1.Unmarshal(deployCreatorMessageRaw, deployCreatorMessage)
	if err != nil {
		cp.client.error("Failed unmarshalling message to user [%s].", err.Error())
		return nil, nil, err
	}
	if deployCreatorMessage.Validate() != nil {
		cp.client.error("Failed validating creator message [%s].", err.Error())
		return nil, nil, err
	}

	skC, err := cp.acSPI.DeserializePrivateKey(deployCreatorMessage.SkC)
	if err != nil {
		cp.client.error("Failed deserializing chaincode secret key [%s].", err.Error())
		return nil, nil, err
	}
	certHandler, err := cp.client.GetTCertificateHandlerFromDER(tx.Cert)
	if err != nil {
		cp.client.error("Failed gettting certificate handler [%s].", err.Error())
		return nil, nil, err
	}

	return skC.GetPublicKey(), certHandler, nil
}

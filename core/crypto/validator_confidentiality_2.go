package crypto

import (
	"encoding/asn1"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
	"reflect"
)

type validatorConfidentialityProcessorV2 struct {
	validator *validatorImpl

	scSPI primitives.StreamCipherSPI
	acSPI primitives.AsymmetricCipherSPI
}

func (cp *validatorConfidentialityProcessorV2) getVersion() string {
	return "2.0"
}

func (cp *validatorConfidentialityProcessorV2) getChaincodeID(ctx TransactionContext) (*obc.ChaincodeID, error) {
	tx := ctx.GetTransaction()

	// 1. Try to unmarshall directly
	cID := &obc.ChaincodeID{}
	err := proto.Unmarshal(tx.ChaincodeID, cID)
	if err != nil {
		cp.validator.debug("Transaction type [%s].", tx.Type.String())

		switch tx.Type {

		case obc.Transaction_CHAINCODE_DEPLOY:
			cp.validator.debug("Extract skC...")

			msgToValidators := new(deployValidatorsMessageV2)
			_, err := asn1.Unmarshal(tx.ToValidators, msgToValidators)
			if err != nil {
				cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
				return nil, err
			}

			aCipher, err := cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
			if err != nil {
				cp.validator.error("Failed init decryption engine [%s].", err.Error())
				return nil, err
			}

			messageToValidatorsChainRaw, err := aCipher.Process(msgToValidators.Chain)
			if err != nil {
				cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
				return nil, err
			}

			messageToValidatorsChain := new(deployValidatorsMessageChainV2)
			_, err = asn1.Unmarshal(messageToValidatorsChainRaw, messageToValidatorsChain)
			if err != nil {
				cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
				return nil, err
			}
			if err := messageToValidatorsChain.Validate(); err != nil {
				return nil, err
			}

			skC, err := cp.validator.acSPI.DeserializePrivateKey(messageToValidatorsChain.SkC)
			if err != nil {
				cp.validator.error("Failed deserializing transaction key [%s].", err.Error())
				return nil, err
			}

			cp.validator.debug("Extract skC...done")

			cp.validator.debug("Extract (kHeader, kCode, kState)...")
			aCipher, err = cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(skC)
			if err != nil {
				cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
				return nil, err
			}

			messageToValidatorsChaincodeRaw, err := aCipher.Process(msgToValidators.Chaincode)
			if err != nil {
				cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
				return nil, err
			}

			messageToValidatorsChaincode := new(deployValidatorsMessageChaincodeV2)
			_, err = asn1.Unmarshal(messageToValidatorsChaincodeRaw, messageToValidatorsChaincode)
			if err != nil {
				cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
				return nil, err
			}
			if err := messageToValidatorsChaincode.Validate(); err != nil {
				return nil, err
			}
			cp.validator.debug("Extract (kHeader, kCode, kState)...done")

			// ChaincodeID has been already decrypted by preValidation
			// Decrypt ChaincodeID using kHeader
			sCipher, err := cp.scSPI.NewStreamCipherForDecryptionFromSerializedKey(messageToValidatorsChaincode.KHeader)
			if err != nil {
				cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
				return nil, err
			}
			chaincodeIDRaw, err := sCipher.Process(tx.ChaincodeID)
			if err != nil {
				cp.validator.error("Failed decrypting chaincode [%s].", err.Error())
				return nil, err
			}
			headerMessage := new(headerMessageV2)
			_, err = asn1.Unmarshal(chaincodeIDRaw, headerMessage)
			if err != nil {
				cp.validator.error("Failed unmarshalling header Message [%s].", err.Error())
				return nil, err
			}
			if err := headerMessage.Validate(); err != nil {
				return nil, err
			}

			err = proto.Unmarshal(headerMessage.ChaincodeID, cID)
			if err != nil {
				cp.validator.error("Failed unmarshalling chaincodeID [%s].", err.Error())
				return nil, err
			}

		case obc.Transaction_CHAINCODE_INVOKE:
		case obc.Transaction_CHAINCODE_QUERY:
			// Decrypt ChaincodeID using skChain
			aCipher, err := cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
			if err != nil {
				cp.validator.error("Failed init decryption engine [%s].", err.Error())
				return nil, err
			}
			chaincodeIDRaw, err := aCipher.Process(tx.ChaincodeID)
			if err != nil {
				cp.validator.error("Failed decrypting chaincode [%s].", err.Error())
				return nil, err
			}
			headerMessage := new(headerMessageV2)
			_, err = asn1.Unmarshal(chaincodeIDRaw, headerMessage)
			if err != nil {
				cp.validator.error("Failed unmarshalling header Message [%s].", err.Error())
				return nil, err
			}
			if err := headerMessage.Validate(); err != nil {
				return nil, err
			}

			err = proto.Unmarshal(headerMessage.ChaincodeID, cID)
			if err != nil {
				cp.validator.error("Failed unmarshalling chaincodeID [%s].", err.Error())
				return nil, err
			}
		}

	}

	return cID, nil
}

func (cp *validatorConfidentialityProcessorV2) preValidation(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()

	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Failed decrypting payload. Invalid nonce.")
	}

	return tx, nil
}

func (cp *validatorConfidentialityProcessorV2) preExecution(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()

	// Clone tx
	tx, err := cp.validator.cloneTransaction(tx)
	if err != nil {
		cp.validator.error("Failed deep cloning [%s].", err.Error())
		return nil, err
	}

	switch tx.Type {
	case obc.Transaction_CHAINCODE_DEPLOY:
		return cp.preExecutionDeploy(ctx)
	case obc.Transaction_CHAINCODE_INVOKE:
		return cp.preExecutionExecute(ctx)
	case obc.Transaction_CHAINCODE_QUERY:
		return cp.preExecutionQuery(ctx)
	}

	return nil, utils.ErrInvalidTransactionType
}

func (cp *validatorConfidentialityProcessorV2) preExecutionDeploy(ctx TransactionContext) (*obc.Transaction, error) {
	clone := ctx.GetTransaction()

	cp.validator.debug("Transaction type [%s].", clone.Type.String())

	cp.validator.debug("Extract skC...")

	msgToValidators := new(deployValidatorsMessageV2)
	_, err := asn1.Unmarshal(clone.ToValidators, msgToValidators)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}

	aCipher, err := cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
	if err != nil {
		cp.validator.error("Failed init decryption engine [%s].", err.Error())
		return nil, err
	}

	messageToValidatorsChainRaw, err := aCipher.Process(msgToValidators.Chain)
	if err != nil {
		cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
		return nil, err
	}

	messageToValidatorsChain := new(deployValidatorsMessageChainV2)
	_, err = asn1.Unmarshal(messageToValidatorsChainRaw, messageToValidatorsChain)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}
	if err := messageToValidatorsChain.Validate(); err != nil {
		return nil, err
	}

	skC, err := cp.validator.acSPI.DeserializePrivateKey(messageToValidatorsChain.SkC)
	if err != nil {
		cp.validator.error("Failed deserializing transaction key [%s].", err.Error())
		return nil, err
	}

	cp.validator.debug("Extract skC...done")

	cp.validator.debug("Extract (kHeader, kCode, kState)...")
	aCipher, err = cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(skC)
	if err != nil {
		cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
		return nil, err
	}

	messageToValidatorsChaincodeRaw, err := aCipher.Process(msgToValidators.Chaincode)
	if err != nil {
		cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
		return nil, err
	}

	messageToValidatorsChaincode := new(deployValidatorsMessageChaincodeV2)
	_, err = asn1.Unmarshal(messageToValidatorsChaincodeRaw, messageToValidatorsChaincode)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}
	if err := messageToValidatorsChaincode.Validate(); err != nil {
		return nil, err
	}
	cp.validator.debug("Extract (kHeader, kCode, kState)...done")

	// ChaincodeID has been already decrypted by preValidation
	// Decrypt ChaincodeID using kHeader
	sCipher, err := cp.scSPI.NewStreamCipherForDecryptionFromSerializedKey(messageToValidatorsChaincode.KHeader)
	if err != nil {
		cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
		return nil, err
	}
	chaincodeIDRaw, err := sCipher.Process(clone.ChaincodeID)
	if err != nil {
		cp.validator.error("Failed decrypting chaincode [%s].", err.Error())
		return nil, err
	}
	headerMessage := new(headerMessageV2)
	_, err = asn1.Unmarshal(chaincodeIDRaw, headerMessage)
	if err != nil {
		cp.validator.error("Failed unmarshalling header Message [%s].", err.Error())
		return nil, err
	}
	if err := headerMessage.Validate(); err != nil {
		return nil, err
	}
	clone.ChaincodeID = headerMessage.ChaincodeID

	// Decrypt Payload using kCode
	sCipher, err = cp.scSPI.NewStreamCipherForDecryptionFromSerializedKey(messageToValidatorsChaincode.KCode)
	if err != nil {
		cp.validator.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}
	payload, err := sCipher.Process(clone.Payload)
	if err != nil {
		cp.validator.error("Failed decrypting payload [%s].", err.Error())
		return nil, err
	}
	clone.Payload = payload

	// Decrypt metadata using kCode
	if len(clone.Metadata) != 0 {
		metadata, err := sCipher.Process(clone.Metadata)
		if err != nil {
			cp.validator.error("Failed decrypting metadata [%s].", err.Error())
			return nil, err
		}
		clone.Metadata = metadata
	}

	return clone, nil
}

func (cp *validatorConfidentialityProcessorV2) preExecutionExecute(ctx TransactionContext) (*obc.Transaction, error) {
	skC, err := cp.getSKCFromDeployTransaction(ctx.GetDeployTransaction())
	if err != nil {
		cp.validator.error("Failed loading skC from deploy transaction [%s].", err.Error())

		return nil, err
	}

	tx := ctx.GetTransaction()

	cp.validator.debug("Transaction type [%s].", tx.Type.String())

	cp.validator.debug("Extract kI...")

	aCipher, err := cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(skC)
	if err != nil {
		cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
		return nil, err
	}

	msgToValidatorsRaw, err := aCipher.Process(tx.ToValidators)
	if err != nil {
		cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
		return nil, err
	}

	msgToValidators := new(eValidatorMessagesV2)
	_, err = asn1.Unmarshal(msgToValidatorsRaw, msgToValidators)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}
	if err := msgToValidators.Validate(); err != nil {
		cp.validator.error("Failed validating message to validators [%s].", err.Error())
		return nil, err
	}

	cp.validator.debug("Extract kI...done")

	// Decrypt ChaincodeID using skChain
	aCipher, err = cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
	if err != nil {
		cp.validator.error("Failed init decryption engine [%s].", err.Error())
		return nil, err
	}
	chaincodeIDRaw, err := aCipher.Process(tx.ChaincodeID)
	if err != nil {
		cp.validator.error("Failed decrypting chaincode [%s].", err.Error())
		return nil, err
	}
	headerMessage := new(headerMessageV2)
	_, err = asn1.Unmarshal(chaincodeIDRaw, headerMessage)
	if err != nil {
		cp.validator.error("Failed unmarshalling header Message [%s].", err.Error())
		return nil, err
	}
	if err := headerMessage.Validate(); err != nil {
		return nil, err
	}
	tx.ChaincodeID = headerMessage.ChaincodeID

	// Decrypt Payload using kI
	sCipher, err := cp.scSPI.NewStreamCipherForDecryptionFromSerializedKey(msgToValidators.KInvoke)
	if err != nil {
		cp.validator.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}

	payloadRaw, err := sCipher.Process(tx.Payload)
	if err != nil {
		cp.validator.error("Failed decrypting payload [%s].", err.Error())
		return nil, err
	}
	payload := new(ePayloadV2)
	_, err = asn1.Unmarshal(payloadRaw, payload)
	if err != nil {
		cp.validator.error("Failed unmarshalling payload [%s].", err.Error())
		return nil, err
	}
	if err := payload.Validate(); err != nil {
		cp.validator.error("Failed validating payload [%s].", err.Error())
		return nil, err
	}

	// Validate cert hash and signature
	txCertHash := primitives.Hash(tx.Cert)
	if !reflect.DeepEqual(txCertHash, payload.TxCertHash) {
		cp.validator.error("Failed validating payload.TxCertHash [% x] != [% x].", txCertHash, payload.TxCertHash)
		return nil, err
	}

	// TODO: Verify TxSign using the certificate specified in the deploy transaction
	//payload.TxSign

	tx.Payload = payload.Payload

	// Decrypt metadata using kI
	if len(tx.Metadata) != 0 {
		metadata, err := sCipher.Process(tx.Metadata)
		if err != nil {
			cp.validator.error("Failed decrypting metadata [%s].", err.Error())
			return nil, err
		}
		tx.Metadata = metadata
	}

	return tx, nil
}

func (cp *validatorConfidentialityProcessorV2) preExecutionQuery(ctx TransactionContext) (*obc.Transaction, error) {
	return cp.preExecutionExecute(ctx)
}

func (cp *validatorConfidentialityProcessorV2) getSKCFromDeployTransaction(tx *obc.Transaction) (primitives.PrivateKey, error) {
	if tx == nil {
		return nil, utils.ErrNilArgument
	}

	deployValidatorsMessage := new(deployValidatorsMessageV2)
	_, err := asn1.Unmarshal(tx.ToValidators, deployValidatorsMessage)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}
	if deployValidatorsMessage.Validate() != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}

	aCipher, err := cp.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
	if err != nil {
		cp.validator.error("Failed creating asymmetric cipher [%s].", err.Error())
		return nil, err
	}

	deployValidatorsMessageChainRaw, err := aCipher.Process(deployValidatorsMessage.Chain)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}
	deployValidatorsMessageChain := new(deployValidatorsMessageChainV2)
	_, err = asn1.Unmarshal(deployValidatorsMessageChainRaw, deployValidatorsMessageChain)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators (chain) [%s].", err.Error())
		return nil, err
	}
	if deployValidatorsMessageChain.Validate() != nil {
		cp.validator.error("Failed unmarshalling message to validators (chain) [%s].", err.Error())
		return nil, err
	}

	skC, err := cp.acSPI.DeserializePrivateKey(deployValidatorsMessageChain.SkC)
	if err != nil {
		cp.validator.error("Failed deserializing skC [%s].", err.Error())
		return nil, err
	}

	return skC, nil
}

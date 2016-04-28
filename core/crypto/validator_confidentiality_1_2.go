package crypto

import (
	"encoding/asn1"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	obc "github.com/hyperledger/fabric/protos"
)

type validatorConfidentialityProcessorV1_2 struct {
	validator *validatorImpl
}

func (cp *validatorConfidentialityProcessorV1_2) getVersion() string {
	return "1.2"
}

func (cp *validatorConfidentialityProcessorV1_2) getChaincodeID(ctx TransactionContext) (*obc.ChaincodeID, error) {
	tx := ctx.GetTransaction()

	// 1. Try to unmarshall directly
	cID := &obc.ChaincodeID{}
	err := proto.Unmarshal(tx.ChaincodeID, cID)
	if err != nil {
		// 2. Try to decrypt first

		if tx.Nonce == nil || len(tx.Nonce) == 0 {
			return nil, errors.New("Nil nonce.")
		}

		var ccPrivateKey primitives.PrivateKey

		cp.validator.debug("Getting ChaincodeID...")

		cp.validator.debug("Transaction type [%s].", tx.Type.String())

		cp.validator.debug("Extract transaction key...")

		// Derive transaction key
		cipher, err := cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
		if err != nil {
			cp.validator.error("Failed init decryption engine [%s].", err.Error())
			return nil, err
		}

		cp.validator.debug("Decrypting message to validators [% x].", tx.ToValidators)

		msgToValidatorsRaw, err := cipher.Process(tx.ToValidators)
		if err != nil {
			cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
			return nil, err
		}

		msgToValidators := new(messageToValidatorsV1_2)
		_, err = asn1.Unmarshal(msgToValidatorsRaw, msgToValidators)
		if err != nil {
			cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
			return nil, err
		}

		cp.validator.debug("Deserializing transaction key [% x].", msgToValidators.PrivateKey)
		ccPrivateKey, err = cp.validator.acSPI.DeserializePrivateKey(msgToValidators.PrivateKey)
		if err != nil {
			cp.validator.error("Failed deserializing transaction key [%s].", err.Error())
			return nil, err
		}

		cp.validator.debug("Extract transaction key...done")

		cipher, err = cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(ccPrivateKey)
		if err != nil {
			cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
			return nil, err
		}

		// Decrypt ChaincodeID
		chaincodeID, err := cipher.Process(tx.ChaincodeID)
		if err != nil {
			cp.validator.error("Failed decrypting chaincode [%s].", err.Error())
			return nil, err
		}

		err = proto.Unmarshal(chaincodeID, cID)
		if err != nil {
			cp.validator.error("Failed unmarshalling chaincodeID [%s].", err.Error())
			return nil, err
		}

		cp.validator.debug("Getting ChaincodeID...done")
	}

	return cID, nil
}

func (cp *validatorConfidentialityProcessorV1_2) preValidation(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()

	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Nil nonce.")
	}

	return tx, nil
}

func (cp *validatorConfidentialityProcessorV1_2) preExecution(ctx TransactionContext) (*obc.Transaction, error) {
	tx := ctx.GetTransaction()

	// Clone tx
	tx, err := cp.validator.cloneTransaction(tx)
	if err != nil {
		cp.validator.error("Failed deep cloning [%s].", err.Error())
		return nil, err
	}

	var ccPrivateKey primitives.PrivateKey

	cp.validator.debug("Transaction type [%s].", tx.Type.String())

	cp.validator.debug("Extract transaction key...")

	// Derive transaction key
	cipher, err := cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(cp.validator.chainPrivateKey)
	if err != nil {
		cp.validator.error("Failed init decryption engine [%s].", err.Error())
		return nil, err
	}

	cp.validator.debug("Decrypting message to validators [% x].", tx.ToValidators)

	msgToValidatorsRaw, err := cipher.Process(tx.ToValidators)
	if err != nil {
		cp.validator.error("Failed decrypting message to validators [%s].", err.Error())
		return nil, err
	}

	msgToValidators := new(messageToValidatorsV1_2)
	_, err = asn1.Unmarshal(msgToValidatorsRaw, msgToValidators)
	if err != nil {
		cp.validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}

	cp.validator.debug("Deserializing transaction key [% x].", msgToValidators.PrivateKey)
	ccPrivateKey, err = cp.validator.acSPI.DeserializePrivateKey(msgToValidators.PrivateKey)
	if err != nil {
		cp.validator.error("Failed deserializing transaction key [%s].", err.Error())
		return nil, err
	}

	cp.validator.debug("Extract transaction key...done")

	cipher, err = cp.validator.acSPI.NewAsymmetricCipherFromPrivateKey(ccPrivateKey)
	if err != nil {
		cp.validator.error("Failed init transaction decryption engine [%s].", err.Error())
		return nil, err
	}
	// Decrypt Payload
	payload, err := cipher.Process(tx.Payload)
	if err != nil {
		cp.validator.error("Failed decrypting payload [%s].", err.Error())
		return nil, err
	}
	tx.Payload = payload

	// ChaincodeID has been already decrypted by preValidation
	// Decrypt ChaincodeID
	chaincodeID, err := cipher.Process(tx.ChaincodeID)
	if err != nil {
		cp.validator.error("Failed decrypting chaincode [%s].", err.Error())
		return nil, err
	}
	tx.ChaincodeID = chaincodeID

	// Decrypt metadata
	if len(tx.Metadata) != 0 {
		metadata, err := cipher.Process(tx.Metadata)
		if err != nil {
			cp.validator.error("Failed decrypting metadata [%s].", err.Error())
			return nil, err
		}
		tx.Metadata = metadata
	}

	return tx, nil
}

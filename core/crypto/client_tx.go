package crypto

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

type transactionContextImpl struct {
	client *clientImpl

	txHandler TransactionHandler
	tCert     TransactionCertificate
	uuid      string
	nonce     []byte

	chaincodeSpec *obc.ChaincodeSpec
	deployTx      *obc.Transaction
	tx            *obc.Transaction
}

func (tc *transactionContextImpl) GetDeployTransaction() *obc.Transaction {
	return tc.deployTx
}

func (tc *transactionContextImpl) GetTransaction() *obc.Transaction {
	return tc.tx
}

func (tc *transactionContextImpl) GetChaincodeSpec() *obc.ChaincodeSpec {
	return tc.chaincodeSpec
}

func (tc *transactionContextImpl) GetBinding() []byte {
	// TODO: handle the error
	binding, _ := tc.txHandler.GetBinding()
	return binding
}

func (tc *transactionContextImpl) createTx(chaincodeSpec *obc.ChaincodeSpec) (err error) {
	tc.chaincodeSpec = chaincodeSpec

	// Copy metadata from ChaincodeSpec
	tc.tx.Metadata = chaincodeSpec.Metadata

	// 1. set and nonce
	if tc.nonce == nil {
		tc.nonce, err = primitives.GetRandomNonce()
		if err != nil {
			tc.client.error("Failed creating nonce [%s].", err.Error())
			return
		}
	}
	tc.tx.Nonce = tc.nonce

	// 2. set confidentiality protocol version
	tc.tx.ConfidentialityProtocolVersion = DefaultConfidentialityProtocolVersion
	if chaincodeSpec.ConfidentialityProtocolVersion != "" {
		ok := tc.client.isConfidentialityProtocolVersionValid(chaincodeSpec.ConfidentialityProtocolVersion)
		if ok {
			tc.tx.ConfidentialityProtocolVersion = chaincodeSpec.ConfidentialityProtocolVersion
		} else {
			tc.client.warning("Invalid confidentiality protocol version [%s]. Using default!", chaincodeSpec.ConfidentialityProtocolVersion)
		}
	} else {
		tc.client.warning("Using default confidentiality protocol version [%s]", chaincodeSpec.ConfidentialityProtocolVersion)
	}

	// 3. set confidentiality level
	// TODO: check that ConfidentialityLevel is a valid value
	tc.tx.ConfidentialityLevel = chaincodeSpec.ConfidentialityLevel

	// 4. Append the certificate to the transaction
	tc.client.debug("Appending certificate [% x].", tc.tCert.GetRaw())
	tc.tx.Cert = tc.tCert.GetRaw()

	// 5. process confidentiality
	err = tc.client.processConfidentiality(tc)
	if err != nil {
		tc.client.error("Failed encrypting payload [%s].", err.Error())
		return

	}

	return
}

func (tc *transactionContextImpl) finalizeTx() (err error) {
	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tc.tx)
	if err != nil {
		tc.client.error("Failed marshaling tx [%s].", err.Error())
		return
	}

	// 2. Sign rawTx and check signature
	tc.client.debug("Signing tx [% x].", rawTx)
	rawSignature, err := tc.tCert.Sign(rawTx)
	if err != nil {
		tc.client.error("Failed creating signature [%s].", err.Error())
		return
	}

	// 3. Append the signature
	tc.tx.Signature = rawSignature

	tc.client.debug("Appending signature: [% x]", rawSignature)

	return
}

func (tc *transactionContextImpl) createDeployTx(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec) (err error) {
	// Create a new transaction
	tc.tx, err = obc.NewChaincodeDeployTransaction(chaincodeDeploymentSpec, tc.uuid)
	if err != nil {
		tc.client.error("Failed creating new transaction [%s].", err.Error())
		return
	}

	err = tc.createTx(chaincodeDeploymentSpec.ChaincodeSpec)

	return
}

func (tc *transactionContextImpl) createExecuteTx(chaincodeInvocation *obc.ChaincodeInvocationSpec) (err error) {
	/// Create a new transaction
	tc.tx, err = obc.NewChaincodeExecute(chaincodeInvocation, tc.uuid, obc.Transaction_CHAINCODE_INVOKE)
	if err != nil {
		tc.client.error("Failed creating new transaction [%s].", err.Error())
		return
	}

	err = tc.createTx(chaincodeInvocation.ChaincodeSpec)

	return
}

func (tc *transactionContextImpl) createQueryTx(chaincodeInvocation *obc.ChaincodeInvocationSpec) (err error) {
	// Create a new transaction
	tc.tx, err = obc.NewChaincodeExecute(chaincodeInvocation, tc.uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		tc.client.error("Failed creating new transaction [%s].", err.Error())
		return
	}

	err = tc.createTx(chaincodeInvocation.ChaincodeSpec)

	return
}

func (tc *transactionContextImpl) newChaincodeDeploy(chaincodeDeploymentSpec *obc.ChaincodeDeploymentSpec) (*obc.Transaction, error) {
	err := tc.createDeployTx(chaincodeDeploymentSpec)
	if err != nil {
		tc.client.error("Failed creating new deploy transaction [%s].", err.Error())
		return nil, err
	}

	err = tc.finalizeTx()
	if err != nil {
		tc.client.error("Failed creating new deploy transaction [%s].", err.Error())
		return nil, err
	}

	return tc.tx, nil
}

func (tc *transactionContextImpl) newChaincodeExecute(chaincodeInvocation *obc.ChaincodeInvocationSpec) (*obc.Transaction, error) {
	err := tc.createExecuteTx(chaincodeInvocation)
	if err != nil {
		tc.client.error("Failed creating new execute transaction [%s].", err.Error())
		return nil, err
	}

	err = tc.finalizeTx()
	if err != nil {
		tc.client.error("Failed creating new execute transaction [%s].", err.Error())
		return nil, err
	}

	return tc.tx, nil
}

func (tc *transactionContextImpl) newChaincodeQuery(chaincodeInvocation *obc.ChaincodeInvocationSpec) (*obc.Transaction, error) {
	err := tc.createQueryTx(chaincodeInvocation)
	if err != nil {
		tc.client.error("Failed creating new query transaction [%s].", err.Error())
		return nil, err
	}

	err = tc.finalizeTx()
	if err != nil {
		tc.client.error("Failed creating new query transaction [%s].", err.Error())
		return nil, err
	}

	return tc.tx, nil
}

func (client *clientImpl) newTransactionContext(txHandler TransactionHandler, txCert TransactionCertificate, uuid string, nonce []byte) *transactionContextImpl {
	return &transactionContextImpl{client, txHandler, txCert, uuid, nonce, nil, nil, nil}
}

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *clientImpl) checkTransaction(tx *obc.Transaction) error {
	if !client.isInitialized {
		return utils.ErrNotInitialized
	}

	if tx.Cert == nil && tx.Signature == nil {
		return utils.ErrTransactionMissingCert
	}

	if tx.Cert != nil && tx.Signature != nil {
		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := primitives.DERToX509Certificate(tx.Cert)
		if err != nil {
			client.error("Failed unmarshalling cert [%s].", err.Error())
			return err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			client.error("Failed marshaling tx [%s].", err.Error())
			return err
		}
		tx.Signature = signature

		// 2. Verify signature
		ver, err := client.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			client.error("Failed marshaling tx [%s].", err.Error())
			return err
		}

		if ver {
			return nil
		}

		return utils.ErrInvalidTransactionSignature
	}

	return utils.ErrTransactionMissingCert
}

func (client *clientImpl) createTransactionNonce() ([]byte, error) {
	nonce, err := primitives.GetRandomNonce()
	if err != nil {
		client.error("Failed creating nonce [%s].", err.Error())
		return nil, err
	}

	return nonce, err
}

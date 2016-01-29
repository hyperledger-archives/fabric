package crypto

import (
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (client *clientImpl) encryptTx(tx *obc.Transaction) error {

	if len(tx.Nonce) == 0 {
		return errors.New("Failed encrypting payload. Invalid nonce.")
	}

	client.log.Debug("confidentiality protocol version [%s]", tx.ConfidentialityProtocolVersion)
	switch tx.ConfidentialityProtocolVersion {
	case "1.1":
		client.log.Debug("Using confidentiality protocol version 1.1")
		return client.encryptTxVersion1_1(tx)
	case "1.2":
		client.log.Debug("Using confidentiality protocol version 1.2")
		return client.encryptTxVersion1_2(tx)
	}

	return utils.ErrInvalidProtocolVersion
}

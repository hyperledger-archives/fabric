package crypto

import (
	"errors"
	"github.com/hyperledger/fabric/core/crypto/primitives/aes"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

func (client *clientImpl) addConfidentialityProcessor(cp ConfidentialityProcessor) (err error) {
	client.confidentialityProcessors[cp.getVersion()] = cp

	return
}

func (client *clientImpl) initConfidentialityProcessors() (err error) {
	client.confidentialityProcessors = make(map[string]ConfidentialityProcessor)

	// Init confidentiality processors
	client.addConfidentialityProcessor(&clientConfidentialityProcessorV1_1{client})
	client.addConfidentialityProcessor(&clientConfidentialityProcessorV1_2{client})
	client.addConfidentialityProcessor(&clientConfidentialityProcessorV2{
		client, aes.NewAES256GSMSPI(), client.acSPI,
	})

	return
}

func (client *clientImpl) isConfidentialityProtocolVersionValid(protocolVersion string) bool {
	_, ok := client.confidentialityProcessors[protocolVersion]
	return ok
}

func (client *clientImpl) processConfidentiality(ctx TransactionContext) (err error) {
	tx := ctx.GetTransaction()

	if tx.ConfidentialityLevel == obc.ConfidentialityLevel_PUBLIC {
		// No confidentiality to be processed
		return
	}

	if len(tx.Nonce) == 0 {
		return errors.New("Failed processing confidentiality. Invalid nonce.")
	}

	client.debug("Confidentiality protocol version [%s]", tx.ConfidentialityProtocolVersion)

	processor, ok := client.confidentialityProcessors[tx.ConfidentialityProtocolVersion]
	if !ok {
		return utils.ErrInvalidProtocolVersion
	}

	_, err = processor.process(ctx)

	return
}

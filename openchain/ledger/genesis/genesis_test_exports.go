package genesis

import (
	"github.com/openblockchain/obc-peer/openchain/crypto"
)

type gen struct {
}

var g := gen{}

func init() {
	crypto.SetGenesis(g)
}

func (gen) MakeGenesis() error {
	return genesis.MakeGenesis(nil)
}


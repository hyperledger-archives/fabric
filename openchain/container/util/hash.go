package util

import (
	"github.com/openblockchain/obc-peer/openchain/util"
)

//name could be ChaincodeID.Name or ChaincodeID.Path
func GenerateHashFromSignature(path string, ctor string, args []string) []byte {
	fargs := ctor
	if args != nil {
		for _, str := range args {
			fargs = fargs + str
		}
	}
	cbytes := []byte(path + fargs)

	b := make([]byte, len(cbytes))
	copy(b, cbytes)
	hash := util.ComputeCryptoHash(b)
	return hash
}

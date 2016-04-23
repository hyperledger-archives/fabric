package generic

import (
	"crypto"
	"crypto/cipher"
	"hash"
)

type Params struct {
	Hash      func() hash.Hash
	hashAlgo  crypto.Hash
	Cipher    func([]byte) (cipher.Block, error)
	BlockSize int
	KeyLen    int
}

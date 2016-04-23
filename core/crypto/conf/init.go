package conf

import (
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"golang.org/x/crypto/sha3"
	"sync"
)

var (
	initOnce sync.Once
)

// Init SHA2
func initSHA2(level int) (err error) { 
	switch level {
		case 256:
			defaultCurve = elliptic.P256()
			defaultHash = sha256.New
		case 384:
			defaultCurve = elliptic.P384()
			defaultHash = sha512.New384
		default:
			err = fmt.Errorf("Security level not supported [%d]", level)
		}
	return
}

// Init SHA3
func initSHA3(level int) (err error) { 
	switch level {
		case 256:
			defaultCurve = elliptic.P256()
			defaultHash = sha3.New256
		case 384:
			defaultCurve = elliptic.P384()
			defaultHash = sha3.New384
		default:
			err = fmt.Errorf("Security level not supported [%d]", level)
		}
	return
}

// Set the security configuration with the hash length and the algorithm  
func SetSecurityLevel(algorithm string , level int) (err error) { 
	switch algorithm {
		case "SHA2":
			err = initSHA2(level)
		case "SHA3":
			err = initSHA3(level)
		default:
			err = fmt.Errorf("Algorithm not supported [%s]", algorithm)
		}
		if err == nil { 
			hashAlgorithm = algorithm
			hashLength = level
		}
	return
}

// InitSecurityLevel initialize the crypto layer at the given security level
func InitSecurityLevel(algorithm string , level int) (err error) {
	initOnce.Do(func() {
		err = SetSecurityLevel(algorithm , level)
	})
	return
}

package utils

import (
	"crypto/elliptic"
	"fmt"
	"github.com/op/go-logging"
	"golang.org/x/crypto/sha3"
	"sync"
)

var (
	initOnce sync.Once
)

func InitSecurityLevel(level int) (err error) {
	logging.MustGetLogger("crypto").Debug("Working at security level [%d]", level)

	initOnce.Do(func() {
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
	})

	return
}

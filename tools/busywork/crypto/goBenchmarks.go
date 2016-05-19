// These benchmark routines are modified versions of benchmarks that appear in
// the Go language source code, and are licensed under the terms of the
// GO_LICENSE that appears in this directory.

package bench

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"os"
	"time"
)

// benchmarkSignP256 returns the number of signs per second
func benchmarkSignP256(n int) int {
	p256 := elliptic.P256()
	hashed := []byte("testing")
	priv, _ := ecdsa.GenerateKey(p256, rand.Reader)

	start := time.Now()

	for i := 0; i < n; i++ {
		_, _, _ = ecdsa.Sign(rand.Reader, priv, hashed)
	}

	return int(float64(n) / time.Since(start).Seconds())
}

// benchmarkVerifyP256 returns the number of verifys per second
func benchmarkVerifyP256(n int) int {
	p256 := elliptic.P256()
	hashed := []byte("testing")
	priv, _ := ecdsa.GenerateKey(p256, rand.Reader)
	r, s, _ := ecdsa.Sign(rand.Reader, priv, hashed)

	start := time.Now()

	for i := 0; i < n; i++ {
		if !ecdsa.Verify(&priv.PublicKey, hashed, r, s) {
			fmt.Printf("Fail\n")
			os.Exit(1)
		}
	}

	return int(float64(n) / time.Since(start).Seconds())
}

// benchmarkSignP384 returns the number of signs per second
func benchmarkSignP384(n int) int {
	p384 := elliptic.P384()
	hashed := []byte("testing")
	priv, _ := ecdsa.GenerateKey(p384, rand.Reader)

	start := time.Now()

	for i := 0; i < n; i++ {
		_, _, _ = ecdsa.Sign(rand.Reader, priv, hashed)
	}

	return int(float64(n) / time.Since(start).Seconds())
}

// benchmarkVerifyP384 returns the number of verifys per second
func benchmarkVerifyP384(n int) int {
	p384 := elliptic.P384()
	hashed := []byte("testing")
	priv, _ := ecdsa.GenerateKey(p384, rand.Reader)
	r, s, _ := ecdsa.Sign(rand.Reader, priv, hashed)

	start := time.Now()

	for i := 0; i < n; i++ {
		if !ecdsa.Verify(&priv.PublicKey, hashed, r, s) {
			fmt.Printf("Fail\n")
			os.Exit(1)
		}
	}

	return int(float64(n) / time.Since(start).Seconds())
}

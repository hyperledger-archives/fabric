// These benchmark routines are modified versions of benchmarks that appear in
// the Go language source code, and are licensed under the terms of the
// GO_LICENSE that appears in this directory.

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"os"
	"time"
)

// P256Sign returns the number of signs per second
func P256Sign(n int) (int, string) {
	p256 := elliptic.P256()
	hashed := []byte("testing")
	priv, _ := ecdsa.GenerateKey(p256, rand.Reader)

	start := time.Now()

	for i := 0; i < n; i++ {
		_, _, _ = ecdsa.Sign(rand.Reader, priv, hashed)
	}

	return int(float64(n) / time.Since(start).Seconds()), "operations"
}

// P256Verify returns the number of verifys per second
func P256Verify(n int) (int, string) {
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

	return int(float64(n) / time.Since(start).Seconds()), "operations"
}

// P384Sign returns the number of signs per second
func P384Sign(n int) (int, string) {
	p384 := elliptic.P384()
	hashed := []byte("testing")
	priv, _ := ecdsa.GenerateKey(p384, rand.Reader)

	start := time.Now()

	for i := 0; i < n; i++ {
		_, _, _ = ecdsa.Sign(rand.Reader, priv, hashed)
	}

	return int(float64(n) / time.Since(start).Seconds()), "operations"
}

// P384Verify returns the number of verifys per second
func P384Verify(n int) (int, string) {
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

	return int(float64(n) / time.Since(start).Seconds()), "operations"
}

var shaBuf = make([]byte, 8192)

var sha256Object = sha256.New()
var sha512Object = sha512.New()

func sha256Bench(size int, n int) int {
	sum := make([]byte, sha256Object.Size())

	start := time.Now()

	for i := 0; i < n; i++ {
		sha256Object.Reset()
		sha256Object.Write(shaBuf[:size])
		sha256Object.Sum(sum[:0])
	}

	return int(float64(size*n) / time.Since(start).Seconds())
}

// SHA256x8 benchmarks SHA256 on 8-byte buffers
func SHA256x8(n int) (int, string) {
	return sha256Bench(8, n), "bytes"
}

// SHA256x1K benchmarks SHA256 on 1024-byte buffers
func SHA256x1K(n int) (int, string) {
	return sha256Bench(1024, n), "bytes"
}

// SHA256x8K benchmarks SHA256 on 8192-byte buffers
func SHA256x8K(n int) (int, string) {
	return sha256Bench(8192, n), "bytes"
}

func sha512Bench(size int, n int) int {
	sum := make([]byte, sha512Object.Size())

	start := time.Now()

	for i := 0; i < n; i++ {
		sha512Object.Reset()
		sha512Object.Write(shaBuf[:size])
		sha512Object.Sum(sum[:0])
	}

	return int(float64(size*n) / time.Since(start).Seconds())
}

// SHA512x8 benchmarks SHA512 on 8-byte buffers
func SHA512x8(n int) (int, string) {
	return sha512Bench(8, n), "bytes"
}

// SHA512x1K benchmarks SHA512 on 1024-byte buffers
func SHA512x1K(n int) (int, string) {
	return sha512Bench(1024, n), "bytes"
}

// SHA512x8K benchmarks SHA512 on 8192-byte buffers
func SHA512x8K(n int) (int, string) {
	return sha512Bench(8192, n), "bytes"
}

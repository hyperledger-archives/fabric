package c25519

import (
	"crypto/rand"
	"fmt"
	"golang.org/x/crypto/curve25519"
	"math/big"
	"testing"
)

const expectedHex = "89161fde887b2b53de549af483940106ecc114d6982daa98256de23bdf77661a"

func TestBaseScalarMult(t *testing.T) {
	var a, b [32]byte
	in := &a
	out := &b
	a[0] = 1

	for i := 0; i < 200; i++ {
		curve25519.ScalarBaseMult(out, in)
		in, out = out, in
	}

	result := fmt.Sprintf("%x", in[:])
	if result != expectedHex {
		t.Errorf("incorrect result: got %s, want %s", result, expectedHex)
	}
}

func TestCurve(t *testing.T) {
	var a, b [32]byte
	var ar, br []byte

	ar = make([]byte, 32)
	br = make([]byte, 32)
	rand.Reader.Read(ar)
	rand.Reader.Read(br)

	copy(a[:], ar)
	copy(b[:], br)

	in := &a
	out := &b
	a[0] = 1

	curve25519.ScalarBaseMult(out, in)
	fmt.Printf("%x\n", in[:])

	aa := new(big.Int)
	aa.SetBytes(a[:])

	bb := new(big.Int)
	bb.SetBytes(b[:])

	var inin, outout [32]byte
	copy(inin[:], aa.Bytes())
	copy(outout[:], bb.Bytes())

	curve25519.ScalarBaseMult(&outout, &inin)
	fmt.Printf("%x\n", inin[:])
}

package c25519

import (
	"math/big"
)

type Curve25519 struct {
}

// Params returns the parameters for the curve.
func (c *Curve25519) Params() *Curve25519 {
	return c
}

// IsOnCurve reports whether the given (x,y) lies on the curve.
func (c *Curve25519) IsOnCurve(x, y *big.Int) bool {
	return false
}

// Add returns the sum of (x1,y1) and (x2,y2)
func (c *Curve25519) Add(x1, y1, x2, y2 *big.Int) (x, y *big.Int) {
	return nil, nil
}

// Double returns 2*(x,y)
func (c *Curve25519) Double(x1, y1 *big.Int) (x, y *big.Int) {
	return nil, nil
}

// ScalarMult returns k*(Bx,By) where k is a number in big-endian form.
func (c *Curve25519) ScalarMult(x1, y1 *big.Int, k []byte) (x, y *big.Int) {
	return nil, nil
}

// ScalarBaseMult returns k*G, where G is the base point of the group
// and k is an integer in big-endian form.
func (c *Curve25519) ScalarBaseMult(k []byte) (x, y *big.Int) {
	return nil, nil
}

package ecies

import (
	"errors"
	"io"
)

var (
	// ErrInvalidKeyParameter Invalid Key Parameter
	ErrInvalidKeyParameter = errors.New("Invalid Key Parameter.")

	// ErrInvalidKeyGeneratorParameter Invalid Key Generator Parameter
	ErrInvalidKeyGeneratorParameter = errors.New("Invalid Key Generator Parameter.")
)

// Parameter is common interface for all the parameters
type Parameter interface {
	GetRand() io.Reader
}

// CipherParameters is common interface to represent cipher parameters
type CipherParameters interface {
	Parameter
}

// AsymmetricKeyParameter is common interface to represent asymmetric cipher parameters
type AsymmetricCipherParameter interface {
	Parameter

	IsPublic() bool
}

// PublicKey is common interface to represent public asymmetric cipher parameters
type PublicKey interface {
	AsymmetricCipherParameter
}

// PrivateKey is common interface to represent private asymmetric cipher parameters
type PrivateKey interface {
	AsymmetricCipherParameter

	GetPublicKey() PublicKey
}

// KeyGeneratorParameter is common interface to represent key generation parameters
type KeyGeneratorParameter interface {
	Parameter
}

// KeyGenerator defines a key generator
type KeyGenerator interface {
	Init(params KeyGeneratorParameter) error

	GenerateKey() (PrivateKey, error)
}

// KeyGenerator defines an asymmetric cipher
type AsymmetricCipher interface {
	// Init initializes this cipher with the passed parameters
	Init(params AsymmetricCipherParameter) error

	// Process processes the byte array given in input
	Process(msg []byte) ([]byte, error)
}

// KeySerializer defines a key serializer/deserializer
type KeySerializer interface {
	// ToBytes converts a key to bytes
	ToBytes(key interface{}) ([]byte, error)

	// ToBytes converts bytes to a key
	FromBytes([]byte) (interface{}, error)
}

// SPI is the ECIES Service Provider Interface
type SPI interface {
	NewAsymmetricCipherFromPrivateKey(priv PrivateKey) (AsymmetricCipher, error)
	NewAsymmetricCipherFromPublicKey(pub PublicKey) (AsymmetricCipher, error)
	NewPrivateKey(rand io.Reader, params interface{}) (PrivateKey, error)
	NewPublicKey(rand io.Reader, params interface{}) (PublicKey, error)
	SerializePrivateKey(priv PrivateKey) ([]byte, error)
	DeserializePrivateKey(bytes []byte) (PrivateKey, error)
}

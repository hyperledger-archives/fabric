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

// Parameters is common interface for all the parameters
type Parameters interface {

	// GetRand returns the random generated associated to this parameters
	GetRand() io.Reader
}

// CipherParameters is common interface to represent cipher parameters
type CipherParameters interface {
	Parameters
}

// AsymmetricCipherParameters is common interface to represent asymmetric cipher parameters
type AsymmetricCipherParameters interface {
	Parameters

	// IsPublic returns true if the parameters are public, false otherwise.
	IsPublic() bool
}

// PublicKey is common interface to represent public asymmetric cipher parameters
type PublicKey interface {
	AsymmetricCipherParameters
}

// PrivateKey is common interface to represent private asymmetric cipher parameters
type PrivateKey interface {
	AsymmetricCipherParameters

	// GetPublicKey returns the associated public key
	GetPublicKey() PublicKey
}

// KeyGeneratorParameters is common interface to represent key generation parameters
type KeyGeneratorParameters interface {
	Parameters
}

// KeyGenerator defines a key generator
type KeyGenerator interface {
	// Init initializes this generated using the passed parameters
	Init(params KeyGeneratorParameters) error

	// GenerateKey generates a new private key
	GenerateKey() (PrivateKey, error)
}

// AsymmetricCipher defines an asymmetric cipher
type AsymmetricCipher interface {
	// Init initializes this cipher with the passed parameters
	Init(params AsymmetricCipherParameters) error

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

	// NewAsymmetricCipherFromPrivateKey creates a new AsymmetricCipher for decryption from a secret key
	NewAsymmetricCipherFromPrivateKey(priv PrivateKey) (AsymmetricCipher, error)

	// NewAsymmetricCipherFromPublicKey creates a new AsymmetricCipher for encryption from a public key
	NewAsymmetricCipherFromPublicKey(pub PublicKey) (AsymmetricCipher, error)

	// NewPrivateKey creates a new private key from (rand, params)
	NewPrivateKey(rand io.Reader, params interface{}) (PrivateKey, error)

	// NewPublicKey creates a new public key from (rand, params)
	NewPublicKey(rand io.Reader, params interface{}) (PublicKey, error)

	// SerializePrivateKey serializes a private key
	SerializePrivateKey(priv PrivateKey) ([]byte, error)

	// DeserializePrivateKey deserializes to a private key
	DeserializePrivateKey(bytes []byte) (PrivateKey, error)
}

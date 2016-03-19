package generic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"io"
)

func newKeyGeneratorParameter(rand io.Reader, curve elliptic.Curve) (ecies.KeyGeneratorParameters, error) {
	return &keyGeneratorParameterImpl{rand, curve, nil}, nil
}

func newKeyGenerator() (ecies.KeyGenerator, error) {
	return &keyGeneratorImpl{}, nil
}

func newKeyGeneratorFromCurve(rand io.Reader, curve elliptic.Curve) (ecies.KeyGenerator, error) {
	kg, err := newKeyGenerator()
	if err != nil {
		return nil, err
	}

	kgp, err := newKeyGeneratorParameter(rand, curve)
	if err != nil {
		return nil, err
	}

	err = kg.Init(kgp)
	if err != nil {
		return nil, err
	}

	return kg, nil
}

func newPublicKeyFromECDSA(pk *ecdsa.PublicKey) (ecies.PublicKey, error) {
	return &publicKeyImpl{pk, rand.Reader, nil}, nil
}

func newPrivateKeyFromECDSA(sk *ecdsa.PrivateKey) (ecies.PrivateKey, error) {
	return &secretKeyImpl{sk, nil, nil, rand.Reader}, nil
}

func serializePrivateKey(priv ecies.PrivateKey) ([]byte, error) {
	serializer := secretKeySerializerImpl{}
	return serializer.ToBytes(priv)
}

func deserializePrivateKey(bytes []byte) (ecies.PrivateKey, error) {
	serializer := secretKeySerializerImpl{}
	priv, err := serializer.FromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return priv.(ecies.PrivateKey), nil
}

func newAsymmetricCipher() (ecies.AsymmetricCipher, error) {
	return &encryptionSchemeImpl{}, nil
}

func newPrivateKey(rand io.Reader, curve elliptic.Curve) (ecies.PrivateKey, error) {
	kg, err := newKeyGeneratorFromCurve(rand, curve)
	if err != nil {
		return nil, err
	}
	return kg.GenerateKey()
}

func newAsymmetricCipherFromPrivateKey(priv ecies.PrivateKey) (ecies.AsymmetricCipher, error) {
	es, err := newAsymmetricCipher()
	if err != nil {
		return nil, err
	}

	err = es.Init(priv)
	if err != nil {
		return nil, err
	}

	return es, nil
}

func newAsymmetricCipherFromPublicKey(pub ecies.PublicKey) (ecies.AsymmetricCipher, error) {
	es, err := newAsymmetricCipher()
	if err != nil {
		return nil, err
	}

	err = es.Init(pub)
	if err != nil {
		return nil, err
	}

	return es, nil
}

// NewSPI returns a new SPI instance
func NewSPI() ecies.SPI {
	return &spiImpl{}
}

type spiImpl struct {
}

func (spi *spiImpl) NewAsymmetricCipherFromPrivateKey(priv ecies.PrivateKey) (ecies.AsymmetricCipher, error) {
	return newAsymmetricCipherFromPrivateKey(priv)
}

func (spi *spiImpl) NewAsymmetricCipherFromPublicKey(pub ecies.PublicKey) (ecies.AsymmetricCipher, error) {
	return newAsymmetricCipherFromPublicKey(pub)
}

func (spi *spiImpl) NewPrivateKey(r io.Reader, params interface{}) (ecies.PrivateKey, error) {
	switch t := params.(type) {
	case *ecdsa.PrivateKey:
		return newPrivateKeyFromECDSA(t)
	case elliptic.Curve:
		if r == nil {
			r = rand.Reader
		}
		return newPrivateKey(r, t)
	default:
		return nil, ecies.ErrInvalidKeyGeneratorParameter
	}
}

func (spi *spiImpl) NewPublicKey(r io.Reader, params interface{}) (ecies.PublicKey, error) {

	switch t := params.(type) {
	case *ecdsa.PublicKey:
		return newPublicKeyFromECDSA(t)
	default:
		return nil, ecies.ErrInvalidKeyGeneratorParameter
	}
}

func (spi *spiImpl) SerializePrivateKey(priv ecies.PrivateKey) ([]byte, error) {
	return serializePrivateKey(priv)
}

func (spi *spiImpl) DeserializePrivateKey(bytes []byte) (ecies.PrivateKey, error) {
	return deserializePrivateKey(bytes)
}

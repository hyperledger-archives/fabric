package generic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"io"
)

func newKeyGeneratorParameter(rand io.Reader, curve elliptic.Curve) (ecies.KeyGeneratorParameter, error) {
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
	fmt.Printf("[%s]\n", sk)
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

func NewCLI() ecies.CLI {
	return &cliImpl{}
}

type cliImpl struct {
}

func (cli *cliImpl) NewAsymmetricCipherFromPrivateKey(priv ecies.PrivateKey) (ecies.AsymmetricCipher, error) {
	return newAsymmetricCipherFromPrivateKey(priv)
}

func (cli *cliImpl) NewAsymmetricCipherFromPublicKey(pub ecies.PublicKey) (ecies.AsymmetricCipher, error) {
	return newAsymmetricCipherFromPublicKey(pub)
}

func (cli *cliImpl) NewPrivateKey(r io.Reader, params interface{}) (ecies.PrivateKey, error) {
	fmt.Printf("NewPrivateKey [%s]\n", params)

	switch t := params.(type) {
	case *ecdsa.PrivateKey:
		fmt.Printf("2 [%s]\n", t)
		return newPrivateKeyFromECDSA(t)
	case elliptic.Curve:
		fmt.Printf("1 [%s]\n", params)
		if r == nil {
			r = rand.Reader
		}
		return newPrivateKey(r, t)
	default:
		return nil, ecies.ErrInvalidKeyGeneratorParameter
	}
}

func (cli *cliImpl) NewPublicKey(r io.Reader, params interface{}) (ecies.PublicKey, error) {

	switch t := params.(type) {
	case *ecdsa.PublicKey:
		return newPublicKeyFromECDSA(t)
	default:
		return nil, ecies.ErrInvalidKeyGeneratorParameter
	}
}

func (cli *cliImpl) SerializePrivateKey(priv ecies.PrivateKey) ([]byte, error) {
	return serializePrivateKey(priv)
}

func (cli *cliImpl) DeserializePrivateKey(bytes []byte) (ecies.PrivateKey, error) {
	return deserializePrivateKey(bytes)
}

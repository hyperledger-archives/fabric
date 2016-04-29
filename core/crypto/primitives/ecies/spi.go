/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package ecies

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"io"
)

func newKeyGeneratorParameter(rand io.Reader, curve elliptic.Curve) (primitives.KeyGeneratorParameters, error) {
	return &keyGeneratorParameterImpl{rand, curve, nil}, nil
}

func newKeyGenerator() (primitives.KeyGenerator, error) {
	return &keyGeneratorImpl{}, nil
}

func newKeyGeneratorFromCurve(rand io.Reader, curve elliptic.Curve) (primitives.KeyGenerator, error) {
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

func newPublicKeyFromECDSA(pk *ecdsa.PublicKey) (primitives.PublicKey, error) {
	return &publicKeyImpl{pk, rand.Reader, nil}, nil
}

func newPrivateKeyFromECDSA(sk *ecdsa.PrivateKey) (primitives.PrivateKey, error) {
	return &secretKeyImpl{sk, nil, nil, rand.Reader}, nil
}

func serializePrivateKey(priv primitives.PrivateKey) ([]byte, error) {
	serializer := secretKeySerializerImpl{}
	return serializer.ToBytes(priv)
}

func deserializePrivateKey(bytes []byte) (primitives.PrivateKey, error) {
	serializer := secretKeySerializerImpl{}
	priv, err := serializer.FromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return priv.(primitives.PrivateKey), nil
}

func serializePublicKey(priv primitives.PublicKey) ([]byte, error) {
	serializer := publicKeySerializerImpl{}
	return serializer.ToBytes(priv)
}

func deserializePublicKey(bytes []byte) (primitives.PublicKey, error) {
	serializer := publicKeySerializerImpl{}
	pub, err := serializer.FromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return pub.(primitives.PublicKey), nil
}

func newAsymmetricCipher() (primitives.AsymmetricCipher, error) {
	return &encryptionSchemeImpl{}, nil
}

func newPrivateKey(rand io.Reader, curve elliptic.Curve) (primitives.PrivateKey, error) {
	kg, err := newKeyGeneratorFromCurve(rand, curve)
	if err != nil {
		return nil, err
	}
	return kg.GenerateKey()
}

func newAsymmetricCipherFromPrivateKey(priv primitives.PrivateKey) (primitives.AsymmetricCipher, error) {
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

func newAsymmetricCipherFromPublicKey(pub primitives.PublicKey) (primitives.AsymmetricCipher, error) {
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
func NewSPI() primitives.AsymmetricCipherSPI {
	return &spiImpl{}
}

type spiImpl struct {
}

func (spi *spiImpl) NewAsymmetricCipherFromPrivateKey(priv primitives.PrivateKey) (primitives.AsymmetricCipher, error) {
	return newAsymmetricCipherFromPrivateKey(priv)
}

func (spi *spiImpl) NewAsymmetricCipherFromPublicKey(pub primitives.PublicKey) (primitives.AsymmetricCipher, error) {
	return newAsymmetricCipherFromPublicKey(pub)
}

func (spi *spiImpl) NewAsymmetricCipherFromSerializedPublicKey(pub []byte) (primitives.AsymmetricCipher, error) {
	pk, err := spi.DeserializePublicKey(pub)
	if err != nil {
		return nil, err
	}
	return newAsymmetricCipherFromPublicKey(pk)
}

func (spi *spiImpl) NewAsymmetricCipherFromSerializedPrivateKey(priv []byte) (primitives.AsymmetricCipher, error) {
	sk, err := spi.DeserializePrivateKey(priv)
	if err != nil {
		return nil, err
	}
	return newAsymmetricCipherFromPrivateKey(sk)
}

func (spi *spiImpl) NewPrivateKey(r io.Reader, params interface{}) (primitives.PrivateKey, error) {
	switch t := params.(type) {
	case *ecdsa.PrivateKey:
		return newPrivateKeyFromECDSA(t)
	case elliptic.Curve:
		if r == nil {
			r = rand.Reader
		}
		return newPrivateKey(r, t)
	default:
		return nil, primitives.ErrInvalidKeyGeneratorParameter
	}
}

func (spi *spiImpl) NewDefaultPrivateKey(r io.Reader) (primitives.PrivateKey, error) {
	return spi.NewPrivateKey(r, primitives.GetDefaultCurve())
}

func (spi *spiImpl) NewPublicKey(r io.Reader, params interface{}) (primitives.PublicKey, error) {

	switch t := params.(type) {
	case *ecdsa.PublicKey:
		return newPublicKeyFromECDSA(t)
	default:
		return nil, primitives.ErrInvalidKeyGeneratorParameter
	}
}

func (spi *spiImpl) SerializePrivateKey(priv primitives.PrivateKey) ([]byte, error) {
	return serializePrivateKey(priv)
}

func (spi *spiImpl) DeserializePrivateKey(bytes []byte) (primitives.PrivateKey, error) {
	return deserializePrivateKey(bytes)
}

func (spi *spiImpl) SerializePublicKey(priv primitives.PublicKey) ([]byte, error) {
	return serializePublicKey(priv)
}

func (spi *spiImpl) DeserializePublicKey(bytes []byte) (primitives.PublicKey, error) {
	return deserializePublicKey(bytes)
}

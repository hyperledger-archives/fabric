package ecies

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	primitives.InitSecurityLevel("SHA3", 256)
}

func TestSPI(t *testing.T) {

	spi := NewSPI()

	ecdsaKey, err := ecdsa.GenerateKey(primitives.GetDefaultCurve(), rand.Reader)

	var a interface{}
	a = ecdsaKey

	switch t := a.(type) {
	case *ecdsa.PrivateKey:
		fmt.Printf("a2 [%s]\n", t)
		break
	case elliptic.Curve:
		fmt.Printf("a1 [%s]\n", t)
		break
	default:
		fmt.Printf("a3 [%s]\n", t)

	}

	fmt.Printf("[%s]\n", ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = x509.MarshalECPrivateKey(ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("[%s]\n", ecdsaKey)

	privateKey, err := spi.NewPrivateKey(nil, ecdsaKey)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("[%s]\n", privateKey.(*secretKeyImpl).priv)
	_, err = x509.MarshalECPrivateKey(privateKey.(*secretKeyImpl).priv)
	if err != nil {
		t.Fatal(err)
	}

	rawKey, err := spi.SerializePrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	privateKey, err = spi.DeserializePrivateKey(rawKey)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt
	cipher, err := spi.NewAsymmetricCipherFromPublicKey(privateKey.GetPublicKey())
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("Hello World!!!")
	ct, err := cipher.Process(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt
	cipher, err = spi.NewAsymmetricCipherFromPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	plain, err := cipher.Process(ct)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(msg, plain) {
		t.Fatal("Decrypted different message")
	}

}

func TestKG(t *testing.T) {
	kg, err := newKeyGenerator()
	if err != nil {
		t.Fatal(err)
	}

	kgparams, err := newKeyGeneratorParameter(rand.Reader, primitives.GetDefaultCurve())
	if err != nil {
		t.Fatal(err)
	}
	err = kg.Init(kgparams)
	if err != nil {
		t.Fatal(err)
	}
	privKey, err := kg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if privKey == nil {
		t.Fatal("Private Key is nil")
	}
}

func TestES(t *testing.T) {
	cipher, err := newAsymmetricCipher()
	if err != nil {
		t.Fatal(err)
	}

	privKey := generateKey()

	// Encrypt
	plaintext := []byte("Hello World!!!")
	err = cipher.Init(privKey.GetPublicKey())
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Process(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt
	err = cipher.Init(privKey)
	if err != nil {
		t.Fatal(err)
	}
	plaintext2, err := cipher.Process(ciphertext)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(plaintext, plaintext2) {
		t.Fatalf("Decryption failed [%s]!=[%s]", string(plaintext), string(plaintext2))
	}
}

func generateKey() primitives.PrivateKey {
	kg, _ := newKeyGenerator()
	kgparams, _ := newKeyGeneratorParameter(rand.Reader, primitives.GetDefaultCurve())
	kg.Init(kgparams)
	privKey, _ := kg.GenerateKey()
	return privKey
}

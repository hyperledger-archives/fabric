/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aes

import (
	"crypto/rand"
	"reflect"
	"testing"

	"github.com/hyperledger/fabric/core/crypto/primitives"
)

func TestAES256GSMSPIGenerateKey(t *testing.T) {
	spi := NewAES256GSMSPI()

	// Generate Key
	key, err := spi.GenerateKey()
	if err != nil {
		t.Fatalf("Failed generating key [%s]", err)
	}
	if key.GetRand() == nil {
		t.Fatalf("Nil random generator in key.")
	}

	// Encrypt
	msg := []byte("Hellow World")
	sc, err := spi.NewStreamCipherForEncryptionFromKey(key)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForEncryptionFromKey [%s]", err)
	}
	ct, err := sc.Process(msg)
	if err != nil {
		t.Fatalf("Failed encrypting plaintext [%s]", err)
	}

	// Decrypt
	sc, err = spi.NewStreamCipherForDecryptionFromKey(key)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForDecryptionFromKey [%s]", err)
	}
	msg2, err := sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}
}

func TestAES256GSMSPIGenerateKeyAndSerialize(t *testing.T) {
	spi := NewAES256GSMSPI()

	// Generate Key and serialize
	key, keySer, err := spi.GenerateKeyAndSerialize()
	if err != nil {
		t.Fatalf("Failed generating and serializing key [%s]", err)
	}
	if key.GetRand() == nil {
		t.Fatalf("Nil random generator in key.")
	}
	if len(keySer) == 0 {
		t.Fatalf("Serialized key length = 0.")
	}

	// Encrypt
	msg := []byte("Hellow World")
	sc, err := spi.NewStreamCipherForEncryptionFromSerializedKey(keySer)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForEncryptionFromSerializedKey [%s]", err)
	}
	ct, err := sc.Process(msg)
	if err != nil {
		t.Fatalf("Failed encrypting plaintext [%s]", err)
	}

	// Decrypt
	sc, err = spi.NewStreamCipherForDecryptionFromKey(key)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForDecryptionFromKey [%s]", err)
	}
	msg2, err := sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}

	sc, err = spi.NewStreamCipherForDecryptionFromSerializedKey(keySer)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForDecryptionFromSerializedKey [%s]", err)
	}
	msg2, err = sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}
}

func TestAES256GSMSPINewStreamCipher(t *testing.T) {
	spi := NewAES256GSMSPI()

	_, err := spi.NewStreamCipherForEncryptionFromKey(nil)
	if err == nil {
		t.Fatalf("NewStreamCipherForEncryptionFromKey should fail on nil")
	}

	_, err = spi.NewStreamCipherForDecryptionFromKey(nil)
	if err == nil {
		t.Fatalf("NewStreamCipherForEncryptionFromKey should fail on nil")
	}

	_, err = spi.NewStreamCipherForEncryptionFromSerializedKey(nil)
	if err == nil {
		t.Fatalf("NewStreamCipherForEncryptionFromKey should fail on nil")
	}

	_, err = spi.NewStreamCipherForDecryptionFromSerializedKey(nil)
	if err == nil {
		t.Fatalf("NewStreamCipherForEncryptionFromKey should fail on nil")
	}

	_, err = spi.NewStreamCipherForEncryptionFromSerializedKey([]byte{0, 1, 2})
	if err == nil {
		t.Fatalf("NewStreamCipherForEncryptionFromKey should fail on invalid serialized key")
	}

	_, err = spi.NewStreamCipherForDecryptionFromSerializedKey([]byte{0, 1, 2})
	if err == nil {
		t.Fatalf("NewStreamCipherForEncryptionFromKey should fail on invalid serialized key")
	}

}

func TestAES256GSMSPIDencryption(t *testing.T) {
	spi := NewAES256GSMSPI()

	// Generate Key
	key, err := spi.GenerateKey()
	if err != nil {
		t.Fatalf("Failed generating key [%s]", err)
	}
	if key.GetRand() == nil {
		t.Fatalf("Nil random generator in key.")
	}

	// Encrypt
	sc, err := spi.NewStreamCipherForDecryptionFromKey(key)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForDecryptionFromKey [%s]", err)
	}
	_, err = sc.Process(nil)
	if err == nil {
		t.Fatalf("Decryption should fail on nil")
	}
	_, err = sc.Process([]byte{0, 1, 2})
	if err == nil {
		t.Fatalf("Decryption should fail on invalid ciphertext")
	}
	randCt, err := primitives.GetRandomBytes(45)
	if err != nil {
		t.Fatalf("Failed generating random bytes [%s]", err)
	}
	_, err = sc.Process(randCt)
	if err == nil {
		t.Fatalf("Decryption should fail on invalid ciphertext")
	}

}

func TestAES256GSMSPI(t *testing.T) {
	spi := NewAES256GSMSPI()

	// Gen Key

	// Invalid Key
	keyRaw, err := primitives.GetRandomBytes(45)
	if err != nil {
		t.Fatalf("Failed generating random bytes [%s]", err)
	}

	key, err := spi.NewSecretKey(rand.Reader, keyRaw)
	if err == nil {
		t.Fatalf("Generating key should fail on invalid key length")
	}

	key, err = spi.NewSecretKey(rand.Reader, rand.Reader)
	if err == nil {
		t.Fatalf("Generating key should fail on invalid key length")
	}

	// Valid Key
	keyRaw, err = primitives.GetRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed generating random bytes [%s]", err)
	}
	key, err = spi.NewSecretKey(rand.Reader, keyRaw)
	if err != nil {
		t.Fatalf("Failed generating key [%s]", err)
	}
	if key.GetRand() == nil {
		t.Fatalf("Nil random generator in key.")
	}

	// Serialize, Deserialize
	keySer, err := spi.SerializeSecretKey(key)
	if err != nil {
		t.Fatalf("Failed serializing key [%s]", err)
	}
	key, err = spi.DeserializeSecretKey(keySer)
	if err != nil {
		t.Fatalf("Failed deserializing key [%s]", err)
	}
	if reflect.DeepEqual(key, keyRaw) {
		t.Fatalf("Deserialization failed to recover the original key [%x][%x]", keyRaw, key)
	}

	// Encrypt
	msg := []byte("Hellow World")
	sc, err := spi.NewStreamCipherForEncryptionFromKey(key)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForEncryptionFromKey [%s]", err)
	}
	ct, err := sc.Process(msg)
	if err != nil {
		t.Fatalf("Failed encrypting plaintext [%s]", err)
	}

	// Decrypt
	sc, err = spi.NewStreamCipherForDecryptionFromKey(key)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForDecryptionFromKey [%s]", err)
	}
	msg2, err := sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}

	sc, err = spi.NewStreamCipherForDecryptionFromSerializedKey(keySer)
	if err != nil {
		t.Fatalf("Failed NewStreamCipherForDecryptionFromSerializedKey [%s]", err)
	}
	msg2, err = sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}

}

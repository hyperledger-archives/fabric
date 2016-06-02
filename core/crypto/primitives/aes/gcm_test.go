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
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"reflect"
	"testing"
)

func TestAES256GSMSPI(t *testing.T) {
	spi := NewAES256GSMSPI()

	// Gen Key
	keyRaw, err := primitives.GetRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed generating random bytes [%s]", err)
	}
	key, err := spi.NewSecretKey(rand.Reader, keyRaw)
	if err != nil {
		t.Fatalf("Failed generating key [%s]", err)
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

	// Encrypt
	msg := []byte("Hellow World")
	sc, err := spi.NewStreamCipherForEncryptionFromKey(key)
	ct, err := sc.Process(msg)
	if err != nil {
		t.Fatalf("Failed encrypting plaintext [%s]", err)
	}

	// Decrypt
	sc, err = spi.NewStreamCipherForDecryptionFromKey(key)
	msg2, err := sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}

	sc, err = spi.NewStreamCipherForDecryptionFromSerializedKey(keySer)
	msg2, err = sc.Process(ct)
	if err != nil {
		t.Fatalf("Failed decrypting ciphertext [%s]", err)
	}

	// Test msg2 is equal to msg
	if !reflect.DeepEqual(msg2, msg) {
		t.Fatalf("Failed decrypting the right value [%x][%x]", msg, msg2)
	}


}
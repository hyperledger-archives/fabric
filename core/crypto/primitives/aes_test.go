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

package primitives

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"testing"
)

func TestCBCEncrypt_EmptyText(t *testing.T) {
	// Encrypt an empty message. Mainly to document
	// a borderline case. Checking as well that the
	// cipher length is as expected.

	key := make([]byte, 32)
	rand.Reader.Read(key)

	t.Log("Generated key: ", key)

	var msg = []byte("")
	t.Log("Message length: ", len(msg))

	cipher, encErr := CBCEncrypt(key, msg)
	if encErr != nil {
		t.Fatalf("Error encrypting message %v", encErr)
	}

	t.Log("Cipher length: ", len(cipher))

	// expected cipher length: aes.BlockSize
	// the first and only block is the IV
	var expectedLength = aes.BlockSize

	if len(cipher) != expectedLength {
		t.Fatalf("Cipher length is wrong. Expected %d, got %d",
			expectedLength, len(cipher))
	}
	t.Log("Cipher: ", cipher)
}

func TestCBCPKCS7Encrypt_EmptyText(t *testing.T) {
	// Encrypt an empty message. Mainly to document
	// a borderline case. Checking as well that the
	// cipher length is as expected.

	key := make([]byte, 32)
	rand.Reader.Read(key)

	t.Log("Generated key: ", key)

	var msg = []byte("")
	t.Log("Message length: ", len(msg))

	cipher, encErr := CBCPKCS7Encrypt(key, msg)
	if encErr != nil {
		t.Fatalf("Error encrypting message %v", encErr)
	}

	t.Log("Cipher length: ", len(cipher))

	// expected cipher length: 32
	// with padding, at least one block gets encrypted
	// the first block is the IV
	var expectedLength = aes.BlockSize + aes.BlockSize

	if len(cipher) != expectedLength {
		t.Fatalf("Cipher length is wrong. Expected %d, got %d",
			expectedLength, len(cipher))
	}
	t.Log("Cipher: ", cipher)
}

func TestCBCPKCS7Encrypt_IVIsRandom(t *testing.T) {
	// Encrypt two times with same key. The first 16 bytes should be
	// different if IV is random.
	key := make([]byte, 32)
	rand.Reader.Read(key)
	t.Log("Key 1", key)

	var msg = []byte("a message to encrypt")

	cipher1, err := CBCPKCS7Encrypt(key, msg)
	if err != nil {
		t.Fatalf("Error encrypting the message.")
	}

	// expecting a different IV if same message is encrypted with same key
	cipher2, err := CBCPKCS7Encrypt(key, msg)
	if err != nil {
		t.Fatalf("Error encrypting the message.")
	}

	iv1 := cipher1[:aes.BlockSize]
	iv2 := cipher2[:aes.BlockSize]

	t.Log("Cipher 1: ", iv1)
	t.Log("Cipher 2: ", iv2)
	t.Log("bytes.Equal: ", bytes.Equal(iv1, iv2))

	if bytes.Equal(iv1, iv2) {
		t.Fatal("Error: ciphers contain identical initialisation vectors.")
	}

}

func TestCBCPKCS7Encrypt_CipherLengthCorrect(t *testing.T) {
	// Check that the cipher lengths are as expected.
	key := make([]byte, 32)
	rand.Reader.Read(key)

	// length of message < aes.BlockSize (16 bytes)
	// --> expected cipher length = IV length (1 block) + 1 block message
	//     =
	var msg = []byte("short message")
	cipher, err := CBCPKCS7Encrypt(key, msg)
	if err != nil {
		t.Fatal("Error encrypting the message.", cipher)
	}

	expectedLength := aes.BlockSize + aes.BlockSize
	if len(cipher) != expectedLength {
		t.Fatalf("Cipher length incorrect: expected %d, got %d", expectedLength, len(cipher))
	}
}

func TestCBCEncryptCBCDecrypt_KeyMismatch(t *testing.T) {

	defer func() {
		recover()
	}()

	key := make([]byte, 32)
	rand.Reader.Read(key)

	decryptionKey := make([]byte, 32)
	copy(decryptionKey, key[:])
	decryptionKey[0] = key[0] + 1

	var msg = []byte("a message to be encrypted")

	encrypted, _ := CBCEncrypt(key, msg)
	decrypted, _ := CBCDecrypt(decryptionKey, encrypted)

	if string(msg[:]) == string(decrypted[:]) {
		t.Fatalf("Encryption->Decryption with different keys shouldn't return original message")
	}

}

func TestCBCEncryptCBCDecrypt(t *testing.T) {
	// Encrypt with CBCEncrypt and Decrypt with CBCDecrypt

	key := make([]byte, 32)
	rand.Reader.Read(key)

	var msg = []byte("a 16 byte messag")

	encrypted, encErr := CBCEncrypt(key, msg)

	if encErr != nil {
		t.Fatalf("Error encrypting message %v", encErr)
	}

	decrypted, dErr := CBCDecrypt(key, encrypted)

	if dErr != nil {
		t.Fatalf("Error encrypting message %v", dErr)
	}

	if string(msg[:]) != string(decrypted[:]) {
		t.Fatalf("Encryption->Decryption with same key should result in original message")
	}

}

func TestCBCPKCS7EncryptCBCPKCS7Decrypt(t *testing.T) {
	// Encrypt with CBCPKCS7Encrypt and Decrypt with CBCPKCS7Decrypt
	// The intent is not to test the implementation of the aes standard
	// library but to verify the code around the calls to aes.

	key := make([]byte, 32)
	rand.Reader.Read(key)

	var msg = []byte("a message with arbitrary length (42 bytes)")

	encrypted, encErr := CBCPKCS7Encrypt(key, msg)

	if encErr != nil {
		t.Fatalf("Error encrypting message %v", encErr)
	}

	decrypted, dErr := CBCPKCS7Decrypt(key, encrypted)

	if dErr != nil {
		t.Fatalf("Error encrypting message %v", dErr)
	}

	if string(msg[:]) != string(decrypted[:]) {
		t.Fatalf("Encryption->Decryption with same key should result in original message")
	}

}

func TestPKCS7Padding(t *testing.T) {
	// Verify the PKCS7 padding, plaintext version that is easier to read.

	// 0 byte message
	msg := []byte("")
	expected := []byte{16, 16, 16, 16,
		16, 16, 16, 16,
		16, 16, 16, 16,
		16, 16, 16, 16}
	result := PKCS7Padding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("Padding error: Expected ", expected, " but got ", result)
	}

	// 1 byte message
	msg = []byte("0")
	expected = []byte{'0', 15, 15, 15,
		15, 15, 15, 15,
		15, 15, 15, 15,
		15, 15, 15, 15}
	result = PKCS7Padding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("Padding error: Expected ", expected, " but got ", result)
	}

	// 2 byte message
	msg = []byte("01")
	expected = []byte{'0', '1', 14, 14,
		14, 14, 14, 14,
		14, 14, 14, 14,
		14, 14, 14, 14}
	result = PKCS7Padding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("Padding error: Expected ", expected, " but got ", result)
	}

	// 3 to aes.BlockSize-1 byte messages
	for i := 3; i < aes.BlockSize; i++ {
		msg := []byte("0123456789ABCDEF")

		result := PKCS7Padding(msg[:i])

		padding := aes.BlockSize - i
		expectedPadding := bytes.Repeat([]byte{byte(padding)}, padding)
		expected = append(msg[:i], expectedPadding...)

		if !bytes.Equal(result, expected) {
			t.Fatal("Padding error: Expected ", expected, " but got ", result)
		}

	}

	// aes.BlockSize length message
	// !! needs to be modified for PR2093
	msg = bytes.Repeat([]byte{byte('x')}, aes.BlockSize)

	result = PKCS7Padding(msg)

	expectedPadding := bytes.Repeat([]byte{byte(aes.BlockSize)},
		aes.BlockSize)
	expected = append(msg, expectedPadding...)

	if len(result) != 2*aes.BlockSize {
		t.Fatal("Padding error: expected the length of the returned slice ",
			"to be 2 times aes.BlockSize")
	}

	if !bytes.Equal(expected, result) {
		t.Fatal("Padding error: Expected ", expected, " but got ", result)
	}

}

func TestPKCS7UnPadding(t *testing.T) {
	// 0 byte message
	expected := []byte("")
	msg := []byte{16, 16, 16, 16,
		16, 16, 16, 16,
		16, 16, 16, 16,
		16, 16, 16, 16}

	result, _ := PKCS7UnPadding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("UnPadding error: Expected ", expected, " but got ", result)
	}

	// 1 byte message
	expected = []byte("0")
	msg = []byte{'0', 15, 15, 15,
		15, 15, 15, 15,
		15, 15, 15, 15,
		15, 15, 15, 15}

	result, _ = PKCS7UnPadding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("UnPadding error: Expected ", expected, " but got ", result)
	}

	// 2 byte message
	expected = []byte("01")
	msg = []byte{'0', '1', 14, 14,
		14, 14, 14, 14,
		14, 14, 14, 14,
		14, 14, 14, 14}

	result, _ = PKCS7UnPadding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("UnPadding error: Expected ", expected, " but got ", result)
	}

	// 3 to aes.BlockSize-1 byte messages
	for i := 3; i < aes.BlockSize; i++ {
		base := []byte("0123456789ABCDEF")

		iPad := aes.BlockSize - i
		padding := bytes.Repeat([]byte{byte(iPad)}, iPad)
		msg = append(base[:i], padding...)

		expected := base[:i]
		result, _ := PKCS7UnPadding(msg)

		if !bytes.Equal(result, expected) {
			t.Fatal("UnPadding error: Expected ", expected, " but got ", result)
		}

	}

	// aes.BlockSize length message
	// !! needs to be modified for PR2093
	expected = bytes.Repeat([]byte{byte('x')}, aes.BlockSize)

	padding := bytes.Repeat([]byte{byte(aes.BlockSize)},
		aes.BlockSize)
	msg = append(expected, padding...)

	result, _ = PKCS7UnPadding(msg)

	if !bytes.Equal(expected, result) {
		t.Fatal("UnPadding error: Expected ", expected, " but got ", result)
	}

}

func TestCBCEncryptCBCPKCS7Decrypt_ExpectingFailure(t *testing.T) {
	// When encrypting a message that does not need padding, i.e. a message
	// whose length is a multiple of the block size, with CBCEncrypt, it
	// cannot be decrypted with CBCPKCS7DEcrypt.
	//
	// The intend of this test is to document this behaviour for clarity.
	//
	// The reason is that the section 10.3 Note 2 in #PKCS7 states that
	// an extra block is appended to the message for padding. Since this
	// extra block is missing when using CBCEncrypt for encryption,
	// CBCPKCS7Decrypt fails.

	key := make([]byte, 32)
	rand.Reader.Read(key)

	var msg = []byte("a 16 byte messag")

	encrypted, encErr := CBCEncrypt(key, msg)

	if encErr != nil {
		t.Fatalf("Error encrypting message %v", encErr)
	}

	decrypted, dErr := CBCPKCS7Decrypt(key, encrypted)

	if dErr != nil {
		// expected behaviour, decryption fails.
		t.Logf("Expected error decrypting message %v, decrypted message = %v", dErr, decrypted)
	}
}

func TestCBCPKCS7EncryptCBCDecrypt_ExpectingCorruptMessage(t *testing.T) {
	// When encrypting a message that does not need padding, i.e. a message
	// whose length is a multiple of the block size, with CBCPKCS7Encrypt, it
	// can be decrypted with CBCDecrypt but the returned message is corrupted.
	//
	// The intend of this test is to document this behaviour for clarity.
	//
	// The reason is that the section 10.3 Note 2 in #PKCS7 states that
	// an extra block is appended to the message for padding. Since this
	// extra block is added when using CBCPKCS7Encrypt for encryption,
	// CBCDecrypt returns the original message plus this extra block.
	//
	// Note:
	// he same applies for messages of arbitrary length.

	key := make([]byte, 32)
	rand.Reader.Read(key)

	//                0123456789ABCDEF
	var msg = []byte("a 16 byte messag")

	encrypted, encErr := CBCPKCS7Encrypt(key, msg)

	if encErr != nil {
		t.Fatalf("Error encrypting message %v", encErr)
	}

	decrypted, dErr := CBCDecrypt(key, encrypted)

	if dErr != nil {
		t.Fatalf("Error encrypting message %v, %v", dErr, decrypted)
	}

	if string(msg[:]) != string(decrypted[:aes.BlockSize]) {
		t.Log("msg: ", msg)
		t.Log("decrypted: ", decrypted[:aes.BlockSize])
		t.Fatalf("Encryption->Decryption with same key should result in original message")
	}

	if !bytes.Equal(decrypted[aes.BlockSize:], bytes.Repeat([]byte{byte(aes.BlockSize)}, aes.BlockSize)) {
		t.Fatal("Expected extra block with padding in encrypted message", decrypted)
	}

}

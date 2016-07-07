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

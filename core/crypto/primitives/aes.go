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
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

const (
	// AESKeyLength is the default AES key length
	AESKeyLength = 32

	// NonceSize is the default NonceSize
	NonceSize = 24
)

// GenAESKey returns a random AES key of length AESKeyLength
func GenAESKey() ([]byte, error) {
	return GetRandomBytes(AESKeyLength)
}

// PKCS7Padding adds padding for any block smaller than AES block size
func PKCS7Padding(message []byte) []byte {
	if len(message)%aes.BlockSize == 0 {
		return message
	}

	padSize := aes.BlockSize - len(message)%aes.BlockSize
	addByte := bytes.Repeat([]byte{byte(padSize)}, padSize)

	return append(message, addByte...)
}

//PKCS7UnPadding removes any padding from plain text
func PKCS7UnPadding(plainText []byte) ([]byte, error) {
	plaintextLen := len(plainText)
	if plaintextLen == 0 {
		return nil, errors.New("length of plaintext is 0")
	}

	removepadSize := int(plainText[plaintextLen-1])
	if removepadSize == 0 || removepadSize > aes.BlockSize {
		return plainText, nil
	}

	paddedLen := plaintextLen - removepadSize
	thepaddedData := plainText[paddedLen:]

	for v := 0; v < removepadSize; v++ {
		if thepaddedData[v] != byte(removepadSize) {
			return nil, errors.New("padding string must match repeated bytes added")
		}
	}

	truemsgLen := plaintextLen - removepadSize

	return plainText[:truemsgLen], nil
}

// AESEncryptCBC encrypts AES block with CBC mode
func AESEncryptCBC(encKey, message []byte) ([]byte, error) {
	message = PKCS7Padding(message)

	cipherBlock, error := aes.NewCipher(encKey)
	if error != nil {
		return nil, error
	}

	blockSize := aes.BlockSize + len(message)
	tmpCiphertext := make([]byte, blockSize)
	randVector := tmpCiphertext[:aes.BlockSize]
	_, error = io.ReadFull(rand.Reader, randVector)

	if error != nil {
		return nil, error
	}

	encMode := cipher.NewCBCEncrypter(cipherBlock, randVector)
	encMode.CryptBlocks(tmpCiphertext[aes.BlockSize:], message)

	return tmpCiphertext, nil
}

// AESDecryptCBC decrypts a AES block with CBC mode
func AESDecryptCBC(decKey, cipherText []byte) ([]byte, error) {
	decBlock, err := aes.NewCipher(decKey)

	if err != nil {
		return nil, err
	}

	if len(cipherText) < aes.BlockSize {
		return nil, errors.New("ciphertext needs to be multiple of 16")
	}

	initVector := cipherText[:aes.BlockSize]
	originalMsg := cipherText[aes.BlockSize:]

	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	decMode := cipher.NewCBCDecrypter(decBlock, initVector)
	decMode.CryptBlocks(originalMsg, cipherText[aes.BlockSize:])

	return PKCS7UnPadding(originalMsg)
}

// CBCPKCS7Encrypt combines CBC encryption and PKCS7 padding
func CBCPKCS7Encrypt(key, src []byte) ([]byte, error) {
	return AESEncryptCBC(key, PKCS7Padding(src))
}

// CBCPKCS7Decrypt combines CBC decryption and PKCS7 unpadding
func CBCPKCS7Decrypt(key, src []byte) ([]byte, error) {
	pt, err := AESDecryptCBC(key, src)
	if err != nil {
		return nil, err
	}

	original, err := PKCS7UnPadding(pt)
	if err != nil {
		return nil, err
	}

	return original, nil
}

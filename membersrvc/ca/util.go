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

package ca

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"errors"
	"io"
	"log"
	mrand "math/rand"
	"time"

	pb "github.com/hyperledger/fabric/membersrvc/protos"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var (
	// Trace is a trace logger.
	Trace *log.Logger
	// Info is an info logger.
	Info *log.Logger
	// Warning is a warning logger.
	Warning *log.Logger
	// Error is an error logger.
	Error *log.Logger
	// Panic is a panic logger.
	Panic *log.Logger
)

// LogInit initializes the various loggers.
//
func LogInit(trace, info, warning, error, panic io.Writer) {
	Trace = log.New(trace, "TRACE: ", log.LstdFlags|log.Lshortfile)
	Info = log.New(info, "INFO: ", log.LstdFlags)
	Warning = log.New(warning, "WARNING: ", log.LstdFlags|log.Lshortfile)
	Error = log.New(error, "ERROR: ", log.LstdFlags|log.Lshortfile)
	Panic = log.New(panic, "PANIC: ", log.LstdFlags|log.Lshortfile)
}

var rnd = mrand.NewSource(time.Now().UnixNano())

func randomString(n int) string {
	b := make([]byte, n)

	for i, cache, remain := n-1, rnd.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rnd.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

//
// MemberRoleToString converts a member role representation from int32 to a string,
// according to the Role enum defined in ca.proto.
//
func MemberRoleToString(role pb.Role) (string, error) {
	roleMap := pb.Role_name

	roleStr := roleMap[int32(role)]
	if roleStr == "" {
		return "", errors.New("Undefined user role passed.")
	}

	return roleStr, nil
}

// PKCS5Pad adds a PKCS5 padding.
//
func PKCS5Pad(src []byte) []byte {
	padding := aes.BlockSize - len(src)%aes.BlockSize
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, pad...)
}

// PKCS5Unpad removes a PKCS5 padding.
//
func PKCS5Unpad(src []byte) []byte {
	len := len(src)
	unpad := int(src[len-1])
	return src[:(len - unpad)]
}

// CBCEncrypt performs an AES CBC encryption.
//
func CBCEncrypt(key, s []byte) ([]byte, error) {
	src := PKCS5Pad(s)

	if len(src)%aes.BlockSize != 0 {
		return nil, errors.New("plaintext length is not a multiple of the block size")
	}

	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	enc := make([]byte, aes.BlockSize+len(src))
	iv := enc[:aes.BlockSize]
	if _, err := io.ReadFull(crand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(blk, iv)
	mode.CryptBlocks(enc[aes.BlockSize:], src)

	return enc, nil
}

// CBCDecrypt performs an AES CBC decryption.
//
func CBCDecrypt(key, src []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(src) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := src[:aes.BlockSize]
	src = src[aes.BlockSize:]

	if len(src)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext length is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(blk, iv)
	mode.CryptBlocks(src, src)

	return PKCS5Unpad(src), nil
}

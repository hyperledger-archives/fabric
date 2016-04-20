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

package ecdsa

import (
	"github.com/hyperledger/fabric/core/crypto/conf"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"testing"
)

func TestSignatureVerifier(t *testing.T) {
	// Create a signature
	conf.SetSecurityLevel("SHA3", 256)

	cert, key, err := utils.NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := utils.ECDSASign(key, message)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate a new SignatureVerifier
	sv := NewX509ECDSASignatureVerifier()

	// Verify the signature
	ok, err := sv.Verify(cert, signature, message)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Signature does not verify")
	}
}

func TestSignatureVerifierSHA2(t *testing.T) {
	// Create a signature
	conf.SetSecurityLevel("SHA2", 256)

	cert, key, err := utils.NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := utils.ECDSASign(key, message)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate a new SignatureVerifier
	sv := NewX509ECDSASignatureVerifier()

	// Verify the signature
	ok, err := sv.Verify(cert, signature, message)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Signature does not verify")
	}
}


func TestSignatureVerifierSHA2_384(t *testing.T) {
	// Create a signature
	conf.SetSecurityLevel("SHA2", 384)

	cert, key, err := utils.NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := utils.ECDSASign(key, message)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate a new SignatureVerifier
	sv := NewX509ECDSASignatureVerifier()

	// Verify the signature
	ok, err := sv.Verify(cert, signature, message)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Signature does not verify")
	}
}

func TestSignatureVerifierSHA3_384(t *testing.T) {
	// Create a signature
	conf.SetSecurityLevel("SHA3", 384)

	cert, key, err := utils.NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := utils.ECDSASign(key, message)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate a new SignatureVerifier
	sv := NewX509ECDSASignatureVerifier()

	// Verify the signature
	ok, err := sv.Verify(cert, signature, message)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Signature does not verify")
	}
}

func TestSignatureVerifierSHA2_512(t *testing.T) {
	// Create a signature
	conf.SetSecurityLevel("SHA2", 512)

	cert, key, err := utils.NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := utils.ECDSASign(key, message)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate a new SignatureVerifier
	sv := NewX509ECDSASignatureVerifier()

	// Verify the signature
	ok, err := sv.Verify(cert, signature, message)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Signature does not verify")
	}
}

func TestSignatureVerifierSHA3_512(t *testing.T) {
	// Create a signature
	conf.SetSecurityLevel("SHA3", 512)

	cert, key, err := utils.NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := utils.ECDSASign(key, message)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiate a new SignatureVerifier
	sv := NewX509ECDSASignatureVerifier()

	// Verify the signature
	ok, err := sv.Verify(cert, signature, message)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Signature does not verify")
	}
}


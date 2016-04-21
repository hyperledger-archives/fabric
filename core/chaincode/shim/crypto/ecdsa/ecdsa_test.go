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
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"testing"
)

func TestSignatureVerifier(t *testing.T) {
	// Create a signature
	primitives.SetSecurityLevel("SHA3", 256)

	cert, key, err := NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := primitives.ECDSASign(key, message)
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
	primitives.SetSecurityLevel("SHA2", 256)

	cert, key, err := NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := primitives.ECDSASign(key, message)
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
	primitives.SetSecurityLevel("SHA2", 384)

	cert, key, err := NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := primitives.ECDSASign(key, message)
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
	primitives.SetSecurityLevel("SHA3", 384)

	cert, key, err := NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := primitives.ECDSASign(key, message)
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
	primitives.SetSecurityLevel("SHA2", 512)

	cert, key, err := NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := primitives.ECDSASign(key, message)
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
	primitives.SetSecurityLevel("SHA3", 512)

	cert, key, err := NewSelfSignedCert()
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("Hello World!")
	signature, err := primitives.ECDSASign(key, message)
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

// NewSelfSignedCert create a self signed certificate
func NewSelfSignedCert() ([]byte, interface{}, error) {
	privKey, err := primitives.NewECDSAKey()
	if err != nil {
		return nil, nil, err
	}

	testExtKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	testUnknownExtKeyUsage := []asn1.ObjectIdentifier{[]int{1, 2, 3}, []int{2, 59, 1}}
	extraExtensionData := []byte("extra extension")
	commonName := "test.example.com"
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Σ Acme Co"},
			Country:      []string{"US"},
			ExtraNames: []pkix.AttributeTypeAndValue{
				{
					Type:  []int{2, 5, 4, 42},
					Value: "Gopher",
				},
				// This should override the Country, above.
				{
					Type:  []int{2, 5, 4, 6},
					Value: "NL",
				},
			},
		},
		NotBefore: time.Unix(1000, 0),
		NotAfter:  time.Unix(100000, 0),

		SignatureAlgorithm: x509.ECDSAWithSHA384,

		SubjectKeyId: []byte{1, 2, 3, 4},
		KeyUsage:     x509.KeyUsageCertSign,

		ExtKeyUsage:        testExtKeyUsage,
		UnknownExtKeyUsage: testUnknownExtKeyUsage,

		BasicConstraintsValid: true,
		IsCA: true,

		OCSPServer:            []string{"http://ocsp.example.com"},
		IssuingCertificateURL: []string{"http://crt.example.com/ca1.crt"},

		DNSNames:       []string{"test.example.com"},
		EmailAddresses: []string{"gopher@golang.org"},
		IPAddresses:    []net.IP{net.IPv4(127, 0, 0, 1).To4(), net.ParseIP("2001:4860:0:2001::68")},

		PolicyIdentifiers:   []asn1.ObjectIdentifier{[]int{1, 2, 3}},
		PermittedDNSDomains: []string{".example.com", "example.com"},

		CRLDistributionPoints: []string{"http://crl1.example.com/ca1.crl", "http://crl2.example.com/ca1.crl"},

		ExtraExtensions: []pkix.Extension{
			{
				Id:    []int{1, 2, 3, 4},
				Value: extraExtensionData,
			},
		},
	}

	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, err
	}

	return cert, privKey, nil
}

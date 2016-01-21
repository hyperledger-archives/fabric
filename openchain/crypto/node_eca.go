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

package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"
	protobuf "google/protobuf"
	"time"

	"crypto/rsa"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
)

func (node *nodeImpl) retrieveECACertsChain(userID string) error {
	// Retrieve ECA certificate and verify it
	ecaCertRaw, err := node.getECACertificate()
	if err != nil {
		node.log.Error("Failed getting ECA certificate [%s].", err.Error())

		return err
	}
	node.log.Debug("ECA certificate [%s].", utils.EncodeBase64(ecaCertRaw))

	// TODO: Test ECA cert againt root CA
	_, err = utils.DERToX509Certificate(ecaCertRaw)
	if err != nil {
		node.log.Error("Failed parsing ECA certificate [%s].", err.Error())

		return err
	}

	// Store ECA cert
	node.log.Debug("Storing ECA certificate for [%s]...", userID)

	if err := node.ks.storeCert(node.conf.getECACertsChainFilename(), ecaCertRaw); err != nil {
		node.log.Error("Failed storing eca certificate [%s].", err.Error())
		return err
	}

	return nil
}

func (node *nodeImpl) retrieveEnrollmentData(enrollID, enrollPWD string) error {
	key, enrollCertRaw, enrollChainKey, err := node.getEnrollmentCertificateFromECA(enrollID, enrollPWD)
	if err != nil {
		node.log.Error("Failed getting enrollment certificate [id=%s]: [%s]", enrollID, err)

		return err
	}
	node.log.Debug("Enrollment certificate [%s].", utils.EncodeBase64(enrollCertRaw))

	node.log.Debug("Storing enrollment data for user [%s]...", enrollID)

	// Store enrollment id
	err = ioutil.WriteFile(node.conf.getEnrollmentIDPath(), []byte(enrollID), 0700)
	if err != nil {
		node.log.Error("Failed storing enrollment certificate [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment key
	if err := node.ks.storePrivateKey(node.conf.getEnrollmentKeyFilename(), key); err != nil {
		node.log.Error("Failed storing enrollment key [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment cert
	if err := node.ks.storeCert(node.conf.getEnrollmentCertFilename(), enrollCertRaw); err != nil {
		node.log.Error("Failed storing enrollment certificate [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment chain key
	if err := node.ks.storeKey(node.conf.getEnrollmentChainKeyFilename(), enrollChainKey); err != nil {
		node.log.Error("Failed storing enrollment chain key [id=%s]: [%s]", enrollID, err)
		return err
	}

	return nil
}

func (node *nodeImpl) loadEnrollmentKey() error {
	node.log.Debug("Loading enrollment key...")

	enrollPrivKey, err := node.ks.loadPrivateKey(node.conf.getEnrollmentKeyFilename())
	if err != nil {
		node.log.Error("Failed loading enrollment private key [%s].", err.Error())

		return err
	}

	node.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (node *nodeImpl) loadEnrollmentCertificate() error {
	node.log.Debug("Loading enrollment certificate...")

	cert, der, err := node.ks.loadCertX509AndDer(node.conf.getEnrollmentCertFilename())
	if err != nil {
		node.log.Error("Failed parsing enrollment certificate [%s].", err.Error())

		return err
	}
	node.enrollCert = cert

	// TODO: move this to retrieve
	pk := node.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(node.enrollPrivKey, pk)
	if err != nil {
		node.log.Error("Failed checking enrollment certificate against enrollment key [%s].", err.Error())

		return err
	}

	// Set node ID
	node.id = utils.Hash(der)
	node.log.Debug("Setting id to [%s].", utils.EncodeBase64(node.id))

	// Set eCertHash
	node.enrollCertHash = utils.Hash(der)
	node.log.Debug("Setting enrollCertHash to [%s].", utils.EncodeBase64(node.enrollCertHash))

	return nil
}

func (node *nodeImpl) loadEnrollmentID() error {
	node.log.Debug("Loading enrollment id at [%s]...", node.conf.getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(node.conf.getEnrollmentIDPath())
	if err != nil {
		node.log.Error("Failed loading enrollment id [%s].", err.Error())

		return err
	}

	// Set enrollment ID
	node.enrollID = string(enrollID)
	node.log.Debug("Setting enrollment id to [%s].", node.enrollID)

	return nil
}

func (node *nodeImpl) loadEnrollmentChainKey() error {
	node.log.Debug("Loading enrollment chain key...")

	enrollChainKey, err := node.ks.loadKey(node.conf.getEnrollmentChainKeyFilename())
	if err != nil {
		node.log.Error("Failed loading enrollment chain key [%s].", err.Error())

		return err
	}
	node.enrollChainKey = enrollChainKey

	return nil
}

func (node *nodeImpl) loadECACertsChain() error {
	node.log.Debug("Loading ECA certificates chain...")

	pem, err := node.ks.loadCert(node.conf.getECACertsChainFilename())
	if err != nil {
		node.log.Error("Failed loading ECA certificates chain [%s].", err.Error())

		return err
	}

	ok := node.rootsCertPool.AppendCertsFromPEM(pem)
	if !ok {
		node.log.Error("Failed appending ECA certificates chain.")

		return errors.New("Failed appending ECA certificates chain.")
	}

	return nil
}

func (node *nodeImpl) getECAClient() (*grpc.ClientConn, obcca.ECAPClient, error) {
	socket, err := grpc.Dial(node.conf.getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		node.log.Error("Failed dailing in [%s].", err.Error())

		return nil, nil, err
	}
	ecaPClient := obcca.NewECAPClient(socket)

	return socket, ecaPClient, nil
}

func (node *nodeImpl) callECAReadCACertificate(ctx context.Context, opts ...grpc.CallOption) (*obcca.Cert, error) {
	// Get an ECA Client
	sock, ecaP, err := node.getECAClient()
	defer sock.Close()

	// Issue the request
	cert, err := ecaP.ReadCACertificate(ctx, &obcca.Empty{}, opts...)
	if err != nil {
		node.log.Error("Failed requesting read certificate [%s].", err.Error())

		return nil, err
	}

	return cert, nil
}

func (node *nodeImpl) callECAReadCertificate(ctx context.Context, in *obcca.ECertReadReq, opts ...grpc.CallOption) (*obcca.CertPair, error) {
	// Get an ECA Client
	sock, ecaP, err := node.getECAClient()
	defer sock.Close()

	// Issue the request
	resp, err := ecaP.ReadCertificatePair(ctx, in, opts...)
	if err != nil {
		node.log.Error("Failed requesting read certificate [%s].", err.Error())

		return nil, err
	}

	return resp, nil
}

func (node *nodeImpl) callECAReadCertificateByHash(ctx context.Context, in *obcca.Hash, opts ...grpc.CallOption) (*obcca.CertPair, error) {
	// Get an ECA Client
	sock, ecaP, err := node.getECAClient()
	defer sock.Close()

	// Issue the request
	resp, err := ecaP.ReadCertificateByHash(ctx, in, opts...)
	if err != nil {
		node.log.Error("Failed requesting read certificate [%s].", err.Error())

		return nil, err
	}

	return &obcca.CertPair{resp.Cert, nil}, nil
}

func (node *nodeImpl) getEnrollmentCertificateFromECA(id, pw string) (interface{}, []byte, []byte, error) {
	// Get a new ECA Client
	sock, ecaP, err := node.getECAClient()
	defer sock.Close()

	// Run the protocol
	signPriv, err := utils.NewECDSAKey()
	if err != nil {
		node.log.Error("Failed generating ECDSA key [%s].", err.Error())

		return nil, nil, nil, err
	}
	signPub, err := x509.MarshalPKIXPublicKey(&signPriv.PublicKey)
	if err != nil {
		node.log.Error("Failed mashalling ECDSA key [%s].", err.Error())

		return nil, nil, nil, err
	}

	encPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		node.log.Error("Failed generating RSA key [%s].", err.Error())

		return nil, nil, nil, err
	}
	encPub, err := x509.MarshalPKIXPublicKey(&encPriv.PublicKey)
	if err != nil {
		node.log.Error("Failed marshalling RSA key [%s].", err.Error())

		return nil, nil, nil, err
	}

	req := &obcca.ECertCreateReq{&protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		&obcca.Identity{id},
		&obcca.Token{Tok: []byte(pw)},
		&obcca.PublicKey{obcca.CryptoType_ECDSA, signPub},
		&obcca.PublicKey{obcca.CryptoType_RSA, encPub},
		nil}

	resp, err := ecaP.CreateCertificatePair(context.Background(), req)
	if err != nil {
		node.log.Error("Failed invoking CreateCertficatePair [%s].", err.Error())

		return nil, nil, nil, err
	}

	out, err := rsa.DecryptPKCS1v15(rand.Reader, encPriv, resp.Tok.Tok)
	if err != nil {
		node.log.Error("Failed decrypting token [%s].", err.Error())

		return nil, nil, nil, err
	}

	req.Tok.Tok = out
	req.Sig = nil

	hash := utils.NewHash()
	raw, _ := proto.Marshal(req)
	hash.Write(raw)

	r, s, err := ecdsa.Sign(rand.Reader, signPriv, hash.Sum(nil))
	if err != nil {
		node.log.Error("Failed signing [%s].", err.Error())

		return nil, nil, nil, err
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	resp, err = ecaP.CreateCertificatePair(context.Background(), req)
	if err != nil {
		node.log.Error("Failed invoking CreateCertificatePair [%s].", err.Error())

		return nil, nil, nil, err
	}

	node.log.Debug("Enrollment certificate for signing [%s]", utils.EncodeBase64(utils.Hash(resp.Certs.Sign)))
	node.log.Debug("Enrollment certificate for encrypting [%s]", utils.EncodeBase64(utils.Hash(resp.Certs.Enc)))

	// Verify pbCert.Cert

	return signPriv, resp.Certs.Sign, resp.Chain.Tok, nil
}

func (node *nodeImpl) getECACertificate() ([]byte, error) {
	// Call eca.ReadCACertificate
	pbCert, err := node.callECAReadCACertificate(context.Background())
	if err != nil {
		node.log.Error("Failed requesting enrollment certificate [%s].", err.Error())

		return nil, err
	}

	// TODO Verify pbCert.Cert
	return pbCert.Cert, nil
}

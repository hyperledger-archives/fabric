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

	"encoding/asn1"
	"errors"
	"github.com/golang/protobuf/proto"
	ecies "github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
)

var (
	// ECertSubjectRole is the ASN1 object identifier of the subject's role.
	ECertSubjectRole = asn1.ObjectIdentifier{2, 1, 3, 4, 5, 6, 7}
)

func (node *nodeImpl) retrieveECACertsChain(userID string) error {
	// Retrieve ECA certificate and verify it
	ecaCertRaw, err := node.getECACertificate()
	if err != nil {
		node.error("Failed getting ECA certificate [%s].", err.Error())

		return err
	}
	node.debug("ECA certificate [% x].", ecaCertRaw)

	// TODO: Test ECA cert againt root CA
	// TODO: check responce.Cert against rootCA
	x509ECACert, err := utils.DERToX509Certificate(ecaCertRaw)
	if err != nil {
		node.error("Failed parsing ECA certificate [%s].", err.Error())

		return err
	}

	// Prepare ecaCertPool
	node.ecaCertPool = x509.NewCertPool()
	node.ecaCertPool.AddCert(x509ECACert)

	// Store ECA cert
	node.debug("Storing ECA certificate for [%s]...", userID)

	if err := node.ks.storeCert(node.conf.getECACertsChainFilename(), ecaCertRaw); err != nil {
		node.error("Failed storing eca certificate [%s].", err.Error())
		return err
	}

	return nil
}

func (node *nodeImpl) retrieveEnrollmentData(enrollID, enrollPWD string) error {
	key, enrollCertRaw, enrollChainKey, err := node.getEnrollmentCertificateFromECA(enrollID, enrollPWD)
	if err != nil {
		node.error("Failed getting enrollment certificate [id=%s]: [%s]", enrollID, err)

		return err
	}
	node.debug("Enrollment certificate [% x].", enrollCertRaw)

	node.debug("Storing enrollment data for user [%s]...", enrollID)

	// Store enrollment id
	err = ioutil.WriteFile(node.conf.getEnrollmentIDPath(), []byte(enrollID), 0700)
	if err != nil {
		node.error("Failed storing enrollment certificate [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment key
	if err := node.ks.storePrivateKey(node.conf.getEnrollmentKeyFilename(), key); err != nil {
		node.error("Failed storing enrollment key [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment cert
	if err := node.ks.storeCert(node.conf.getEnrollmentCertFilename(), enrollCertRaw); err != nil {
		node.error("Failed storing enrollment certificate [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Code for confidentiality 1.1
	//if err := node.ks.storeKey(node.conf.getEnrollmentChainKeyFilename(), enrollChainKey); err != nil {
	//	node.error("Failed storing enrollment chain key [id=%s]: [%s]", enrollID, err)
	//	return err

	// Code for confidentiality 1.2
	// Store enrollment chain key
	if node.eType == NodeValidator {
		node.debug("Enrollment chain key for validator [%s]...", enrollID)
		// enrollChainKey is a secret key

		node.debug("key [%s]...", string(enrollChainKey))

		key, err := utils.PEMtoPrivateKey(enrollChainKey, nil)
		if err != nil {
			node.error("Failed unmarshalling enrollment chain key [id=%s]: [%s]", enrollID, err)
			return err
		}

		if err := node.ks.storePrivateKey(node.conf.getEnrollmentChainKeyFilename(), key); err != nil {
			node.error("Failed storing enrollment chain key [id=%s]: [%s]", enrollID, err)
			return err
		}
	} else {
		node.debug("Enrollment chain key for non-validator [%s]...", enrollID)
		// enrollChainKey is a public key

		key, err := utils.PEMtoPublicKey(enrollChainKey, nil)
		if err != nil {
			node.error("Failed unmarshalling enrollment chain key [id=%s]: [%s]", enrollID, err)
			return err
		}
		node.debug("Key decoded from PEM [%s]...", enrollID)

		if err := node.ks.storePublicKey(node.conf.getEnrollmentChainKeyFilename(), key); err != nil {
			node.error("Failed storing enrollment chain key [id=%s]: [%s]", enrollID, err)
			return err
		}
	}

	return nil
}

func (node *nodeImpl) loadEnrollmentKey() error {
	node.debug("Loading enrollment key...")

	enrollPrivKey, err := node.ks.loadPrivateKey(node.conf.getEnrollmentKeyFilename())
	if err != nil {
		node.error("Failed loading enrollment private key [%s].", err.Error())

		return err
	}

	node.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (node *nodeImpl) loadEnrollmentCertificate() error {
	node.debug("Loading enrollment certificate...")

	cert, der, err := node.ks.loadCertX509AndDer(node.conf.getEnrollmentCertFilename())
	if err != nil {
		node.error("Failed parsing enrollment certificate [%s].", err.Error())

		return err
	}
	node.enrollCert = cert

	// TODO: move this to retrieve
	pk := node.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(node.enrollPrivKey, pk)
	if err != nil {
		node.error("Failed checking enrollment certificate against enrollment key [%s].", err.Error())

		return err
	}

	// Set node ID
	node.id = utils.Hash(der)
	node.debug("Setting id to [% x].", node.id)

	// Set eCertHash
	node.enrollCertHash = utils.Hash(der)
	node.debug("Setting enrollCertHash to [% x].", node.enrollCertHash)

	return nil
}

func (node *nodeImpl) loadEnrollmentID() error {
	node.debug("Loading enrollment id at [%s]...", node.conf.getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(node.conf.getEnrollmentIDPath())
	if err != nil {
		node.error("Failed loading enrollment id [%s].", err.Error())

		return err
	}

	// Set enrollment ID
	node.enrollID = string(enrollID)
	node.debug("Setting enrollment id to [%s].", node.enrollID)

	return nil
}

func (node *nodeImpl) loadEnrollmentChainKey() error {
	node.debug("Loading enrollment chain key...")

	// Code for confidentiality 1.1
	//enrollChainKey, err := node.ks.loadKey(node.conf.getEnrollmentChainKeyFilename())
	//if err != nil {
	//	node.error("Failed loading enrollment chain key [%s].", err.Error())
	//
	//	return err
	//}
	//node.enrollChainKey = enrollChainKey

	// Code for confidentiality 1.1
	if node.eType == NodeValidator {
		// enrollChainKey is a secret key
		enrollChainKey, err := node.ks.loadPrivateKey(node.conf.getEnrollmentChainKeyFilename())
		if err != nil {
			node.error("Failed loading enrollment chain key: [%s]", err)
			return err
		}
		node.enrollChainKey = enrollChainKey
	} else {
		// enrollChainKey is a public key
		enrollChainKey, err := node.ks.loadPublicKey(node.conf.getEnrollmentChainKeyFilename())
		if err != nil {
			node.error("Failed load enrollment chain key: [%s]", err)
			return err
		}
		node.enrollChainKey = enrollChainKey
	}

	return nil
}

func (node *nodeImpl) loadECACertsChain() error {
	node.debug("Loading ECA certificates chain...")

	pem, err := node.ks.loadCert(node.conf.getECACertsChainFilename())
	if err != nil {
		node.error("Failed loading ECA certificates chain [%s].", err.Error())

		return err
	}

	ok := node.ecaCertPool.AppendCertsFromPEM(pem)
	if !ok {
		node.error("Failed appending ECA certificates chain.")

		return errors.New("Failed appending ECA certificates chain.")
	}

	return nil
}

func (node *nodeImpl) getECAClient() (*grpc.ClientConn, obcca.ECAPClient, error) {
	node.debug("Getting ECA client...")

	conn, err := node.getClientConn(node.conf.getECAPAddr(), node.conf.getECAServerName())
	if err != nil {
		node.error("Failed getting client connection: [%s]", err)
	}

	client := obcca.NewECAPClient(conn)

	node.debug("Getting ECA client...done")

	return conn, client, nil
}

func (node *nodeImpl) callECAReadCACertificate(ctx context.Context, opts ...grpc.CallOption) (*obcca.Cert, error) {
	// Get an ECA Client
	sock, ecaP, err := node.getECAClient()
	defer sock.Close()

	// Issue the request
	cert, err := ecaP.ReadCACertificate(ctx, &obcca.Empty{}, opts...)
	if err != nil {
		node.error("Failed requesting read certificate [%s].", err.Error())

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
		node.error("Failed requesting read certificate [%s].", err.Error())

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
		node.error("Failed requesting read certificate [%s].", err.Error())

		return nil, err
	}

	return &obcca.CertPair{Sign: resp.Cert, Enc: nil}, nil
}

func (node *nodeImpl) getEnrollmentCertificateFromECA(id, pw string) (interface{}, []byte, []byte, error) {
	// Get a new ECA Client
	sock, ecaP, err := node.getECAClient()
	defer sock.Close()

	// Run the protocol

	signPriv, err := utils.NewECDSAKey()
	if err != nil {
		node.error("Failed generating ECDSA key [%s].", err.Error())

		return nil, nil, nil, err
	}
	signPub, err := x509.MarshalPKIXPublicKey(&signPriv.PublicKey)
	if err != nil {
		node.error("Failed mashalling ECDSA key [%s].", err.Error())

		return nil, nil, nil, err
	}

	encPriv, err := utils.NewECDSAKey()
	if err != nil {
		node.error("Failed generating Encryption key [%s].", err.Error())

		return nil, nil, nil, err
	}
	encPub, err := x509.MarshalPKIXPublicKey(&encPriv.PublicKey)
	if err != nil {
		node.error("Failed marshalling Encryption key [%s].", err.Error())

		return nil, nil, nil, err
	}

	req := &obcca.ECertCreateReq{
		Ts:   &protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:   &obcca.Identity{Id: id},
		Tok:  &obcca.Token{Tok: []byte(pw)},
		Sign: &obcca.PublicKey{Type: obcca.CryptoType_ECDSA, Key: signPub},
		Enc:  &obcca.PublicKey{Type: obcca.CryptoType_ECDSA, Key: encPub},
		Sig:  nil}

	resp, err := ecaP.CreateCertificatePair(context.Background(), req)
	if err != nil {
		node.error("Failed invoking CreateCertficatePair [%s].", err.Error())

		return nil, nil, nil, err
	}

	//out, err := rsa.DecryptPKCS1v15(rand.Reader, encPriv, resp.Tok.Tok)
	spi := ecies.NewSPI()
	eciesKey, err := spi.NewPrivateKey(nil, encPriv)
	if err != nil {
		node.error("Failed parsing decrypting key [%s].", err.Error())

		return nil, nil, nil, err
	}

	ecies, err := spi.NewAsymmetricCipherFromPublicKey(eciesKey)
	if err != nil {
		node.error("Failed creating asymmetrinc cipher [%s].", err.Error())

		return nil, nil, nil, err
	}

	out, err := ecies.Process(resp.Tok.Tok)
	if err != nil {
		node.error("Failed decrypting toke [%s].", err.Error())

		return nil, nil, nil, err
	}

	req.Tok.Tok = out
	req.Sig = nil

	hash := utils.NewHash()
	raw, _ := proto.Marshal(req)
	hash.Write(raw)

	r, s, err := ecdsa.Sign(rand.Reader, signPriv, hash.Sum(nil))
	if err != nil {
		node.error("Failed signing [%s].", err.Error())

		return nil, nil, nil, err
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{Type: obcca.CryptoType_ECDSA, R: R, S: S}

	resp, err = ecaP.CreateCertificatePair(context.Background(), req)
	if err != nil {
		node.error("Failed invoking CreateCertificatePair [%s].", err.Error())

		return nil, nil, nil, err
	}

	// Verify response

	// Verify cert for signing
	node.debug("Enrollment certificate for signing [% x]", utils.Hash(resp.Certs.Sign))

	x509SignCert, err := utils.DERToX509Certificate(resp.Certs.Sign)
	if err != nil {
		node.error("Failed parsing signing enrollment certificate for signing: [%s]", err)

		return nil, nil, nil, err
	}

	_, err = utils.GetCriticalExtension(x509SignCert, ECertSubjectRole)
	if err != nil {
		node.error("Failed parsing ECertSubjectRole in enrollment certificate for signing: [%s]", err)

		return nil, nil, nil, err
	}

	err = utils.CheckCertAgainstSKAndRoot(x509SignCert, signPriv, node.ecaCertPool)
	if err != nil {
		node.error("Failed checking signing enrollment certificate for signing: [%s]", err)

		return nil, nil, nil, err
	}

	// Verify cert for encrypting
	node.debug("Enrollment certificate for encrypting [% x]", utils.Hash(resp.Certs.Enc))

	x509EncCert, err := utils.DERToX509Certificate(resp.Certs.Enc)
	if err != nil {
		node.error("Failed parsing signing enrollment certificate for encrypting: [%s]", err)

		return nil, nil, nil, err
	}

	_, err = utils.GetCriticalExtension(x509EncCert, ECertSubjectRole)
	if err != nil {
		node.error("Failed parsing ECertSubjectRole in enrollment certificate for encrypting: [%s]", err)

		return nil, nil, nil, err
	}

	err = utils.CheckCertAgainstSKAndRoot(x509EncCert, encPriv, node.ecaCertPool)
	if err != nil {
		node.error("Failed checking signing enrollment certificate for encrypting: [%s]", err)

		return nil, nil, nil, err
	}

	// END
	node.debug("chain key: [% x]", resp.Chain.Tok)

	return signPriv, resp.Certs.Sign, resp.Pkchain, nil
}

func (node *nodeImpl) getECACertificate() ([]byte, error) {
	responce, err := node.callECAReadCACertificate(context.Background())
	if err != nil {
		node.error("Failed requesting ECA certificate [%s].", err.Error())

		return nil, err
	}

	return responce.Cert, nil
}

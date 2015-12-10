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

package client

import (
	obcca "github.com/openblockchain/obc-peer/obcca/protos"

	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/x509"
	"errors"
	"github.com/golang/protobuf/proto"
	_ "github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google/protobuf"
	"io/ioutil"
	"math/big"
	"time"
)

func (client *clientImpl) retrieveTCACertsChain(userId string) error {
	// Retrieve TCA certificate and verify it
	tcaCertRaw, err := client.getTCACertificate()
	if err != nil {
		client.log.Error("Failed getting TCA certificate %s", err)

		return err
	}
	client.log.Info("Register:TCAcert %s", utils.EncodeBase64(tcaCertRaw))

	// TODO: Test TCA cert againt root CA
	_, err = utils.DERToX509Certificate(tcaCertRaw)
	if err != nil {
		client.log.Error("Failed parsing TCA certificate %s", err)

		return err
	}

	// Store TCA cert
	client.log.Info("Storing TCA certificate for validator [%s]...", userId)

	err = ioutil.WriteFile(client.conf.getTCACertsChainPath(), utils.DERCertToPEM(tcaCertRaw), 0700)
	if err != nil {
		client.log.Error("Failed storing tca certificate: %s", err)
		return err
	}

	return nil
}

func (client *clientImpl) loadTCACertsChain() error {
	// Load TCA certs chain
	client.log.Info("Loading TCA certificates chain at %s...", client.conf.getTCACertsChainPath())

	chain, err := ioutil.ReadFile(client.conf.getTCACertsChainPath())
	if err != nil {
		client.log.Error("Failed loading TCA certificates chain : %s", err.Error())

		return err
	}

	ok := client.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		client.log.Error("Failed appending TCA certificates chain.")

		return errors.New("Failed appending TCA certificates chain.")
	}

	return nil
}

func (client *clientImpl) callTCAReadCertificate(ctx context.Context, in *obcca.TCertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(client.conf.getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		client.log.Error("Failed tca dial in: %s", err)

		return nil, err
	}
	defer sockP.Close()

	tcaP := obcca.NewTCAPClient(sockP)

	cert, err := tcaP.ReadCertificate(context.Background(), in)
	if err != nil {
		client.log.Error("Failed requesting tca read certificate: %s", err)

		return nil, err
	}

	return cert, nil
}

func (client *clientImpl) getTCACertificate() ([]byte, error) {
	client.log.Info("getTCACertificate...")

	// Prepare the request
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertReadReq{&timestamp, &obcca.Identity{Id: "tca-root"}, nil}
	pbCert, err := client.callTCAReadCertificate(context.Background(), req)
	if err != nil {
		client.log.Error("Failed requesting tca certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert

	client.log.Info("getTCACertificate...done!")

	return pbCert.Cert, nil
}

// getNextTCert returns the next available (not yet used) transaction certificate
// corresponding to the tuple (cert, signing key)
func (client *clientImpl) getNextTCert() ([]byte, interface{}, error) {
	client.log.Info("Getting next TCert...")
	rawCert, rawKey, err := client.ks.GetNextTCert(client.getTCertsFromTCA)
	if err != nil {
		client.log.Error("getNextTCert: failed accessing db: %s", err)

		return nil, nil, err
	}

	// rawCert and rawKey are supposed to have been already verified at this point.
	client.log.Info("getNextTCert:cert %s", utils.EncodeBase64(rawCert))
	//	client.log.Info("getNextTCert:key %s", utils.EncodeBase64(rawKey))

	signKey, err := utils.DERToPrivateKey(rawKey)
	if err != nil {
		client.log.Error("getNextTCert: failed parsing key: %s", err)

		return nil, nil, err
	}

	client.log.Info("Getting next TCert...done!")

	return rawCert, signKey, nil
}

func (client *clientImpl) signWithTCert(tCert *x509.Certificate, msg []byte) ([]byte, error) {
	// TODO: to be implemented

	// 1. Extract secret key from the transaction certificate

	// 2. Sign msg with the extracted signing key

	return nil, nil
}

func (client *clientImpl) getTCertsFromTCA(num int) ([][]byte, [][]byte, error) {
	client.log.Info("Get [%d] certificates from the TCA...", num)

	// Contact the TCA
	TCertOwnerKDFKey, derBytes, err := client.tcaCreateCertificateSet(num)
	if err != nil {
		client.log.Debug("Failed contacting TCA %s", err)

		return nil, nil, err
	}

	// Validate the Certificates obtained
	opts := x509.VerifyOptions{
		//		DNSName: "test.example.com",
		Roots: client.rootsCertPool,
	}

	TCertOwnerEncryptKey := utils.HMACTruncated(TCertOwnerKDFKey, []byte{1}, utils.AES_KEY_LENGTH_BYTES)
	ExpansionKey := utils.HMAC(TCertOwnerKDFKey, []byte{2})

	resCert := make([][]byte, num)
	resKeys := make([][]byte, num)

	j := 0
	for i := 0; i < num; i++ {
		client.log.Info("Validating certificate. Index [%d]...", i)
		client.log.Debug("Validating certificate [%s]", utils.EncodeBase64(derBytes[i]))

		certificate, err := utils.DERToX509Certificate(derBytes[i])
		if err != nil {
			client.log.Debug("Failed parsing certificate bytes [%s]", utils.EncodeBase64(derBytes[i]))

			continue
		}

		// TODO: Verify certificate against root certs
		_, err = certificate.Verify(opts) // TODO: do something with chain of certificate given in output
		if err != nil {
			client.log.Error("Failed verifing certificate bytes %s", err)

			//			continue
		}

		// Verify public key

		// 384-bit ExpansionValue = HMAC(Expansion_Key, TCertIndex)
		// Let TCertIndex = Timestamp, RandValue, 1,2,…
		// Timestamp assigned, RandValue assigned and counter reinitialized to 1 per batch

		// TODO: retrieve TCertIndex from the ciphertext encrypted under the TCertOwnerEncryptKey
		ct, err := utils.GetExtension(certificate, utils.TCERT_ENC_TCERTINDEX)
		if err != nil {
			client.log.Error("Failed getting extension TCERT_ENC_TCERTINDEX: %s", err)
			//
			continue
		}

		// Decrypt ct to TCertIndex || EnrollPub_Key || EnrollID
		pt, err := utils.CBCPKCS7Decrypt(TCertOwnerEncryptKey, ct)
		if err != nil {
			client.log.Error("Failed decrypting extension TCERT_ENC_TCERTINDEX: %s", err)

			continue
		}

		// Compute ExpansionValue based on TCertIndex
		TCertIndex := pt
		//		TCertIndex := []byte(strconv.Itoa(i))

		client.log.Info("TCertIndex: %s", TCertIndex)
		mac := hmac.New(utils.NewHash, ExpansionKey)
		mac.Write(TCertIndex)
		ExpansionValue := mac.Sum(nil)

		// Derive tpk and tsk accordingly to ExapansionValue from enrollment pk,sk
		// Computable by TCA / Auditor: TCertPub_Key = EnrollPub_Key + ExpansionValue G
		// using elliptic curve point addition per NIST FIPS PUB 186-4- specified P-384

		// Compute temporary secret key
		tempSK := &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: client.enrollPrivKey.Curve,
				X:     new(big.Int),
				Y:     new(big.Int),
			},
			D: new(big.Int),
		}

		var k = new(big.Int).SetBytes(ExpansionValue)
		var one = new(big.Int).SetInt64(1)
		n := new(big.Int).Sub(client.enrollPrivKey.Params().N, one)
		k.Mod(k, n)
		k.Add(k, one)

		tempSK.D.Add(client.enrollPrivKey.D, k)
		tempSK.D.Mod(tempSK.D, client.enrollPrivKey.PublicKey.Params().N)

		// Compute temporary public key
		tempX, tempY := client.enrollPrivKey.PublicKey.ScalarBaseMult(k.Bytes())
		tempSK.PublicKey.X, tempSK.PublicKey.Y =
			tempSK.PublicKey.Add(
				client.enrollPrivKey.PublicKey.X, client.enrollPrivKey.PublicKey.Y,
				tempX, tempY,
			)

		// Verify temporary public key is a valid point on the reference curve
		isOn := tempSK.Curve.IsOnCurve(tempSK.PublicKey.X, tempSK.PublicKey.Y)
		if !isOn {
			client.log.Error("Failed temporary public key IsOnCurve check.")

			continue
		}

		// Check that the derived public key is the same as the one in the certificate
		certPK := certificate.PublicKey.(*ecdsa.PublicKey)

		cmp := certPK.X.Cmp(tempSK.PublicKey.X)
		if cmp != 0 {
			client.log.Error("Derived public key is different on X")

			continue
		}

		cmp = certPK.Y.Cmp(tempSK.PublicKey.Y)
		if cmp != 0 {
			client.log.Error("Derived public key is different on Y")

			continue
		}

		// Verify the signing capability of tempSK
		err = utils.VerifySignCapability(tempSK, certificate.PublicKey)
		if err != nil {
			client.log.Error("Failed verifing signing capability: %s", err)

			continue
		}

		// Marshall certificate and secret key to be stored in the database
		resCert[j] = derBytes[i]
		resKeys[j], err = utils.PrivateKeyToDER(tempSK)
		if err != nil {
			client.log.Error("Failed marshalling private key: %s", err)

			continue
		}

		//		client.log.Debug("key %s", utils.EncodeBase64(resKeys[j]))
		client.log.Info("Sub index [%d]", j)
		j++
		client.log.Info("Certificate [%d] validated.", i)
	}

	if j == 0 {
		client.log.Error("No valid TCert was sent")

		return nil, nil, errors.New("No valid TCert was sent.")
	}

	return resCert[:j], resKeys[:j], nil
}

func (client *clientImpl) tcaCreateCertificateSet(num int) ([]byte, [][]byte, error) {
	//		return mockTcaCreateCertificates(&client.enrollPrivKey.PublicKey, num)

	sockP, err := grpc.Dial(client.conf.getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		client.log.Error("Failed tca dial in: %s", err)

		return nil, nil, err
	}
	defer sockP.Close()

	tcaP := obcca.NewTCAPClient(sockP)

	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertCreateSetReq{
		&timestamp,
		&obcca.Identity{Id: client.enrollId},
		uint32(num),
		nil,
	}
	rawReq, err := proto.Marshal(req)
	if err != nil {
		client.log.Error("Failed marshaling request %s:", err)
		return nil, nil, err
	}

	// 2. Sign rawReq and (TODO) check signature
	client.log.Info("Signing req %s", utils.EncodeBase64(rawReq))
	r, s, err := client.ecdsaSignWithEnrollmentKey(rawReq)
	if err != nil {
		client.log.Error("Failed creating signature %s:", err)
		return nil, nil, err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	// 3. Append the signature
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	// 4. Send request
	certSet, err := tcaP.CreateCertificateSet(context.Background(), req)
	if err != nil {
		client.log.Error("Failed requesting tca create certificate set: %s", err)

		return nil, nil, err
	}

	return certSet.Key, certSet.Certs, nil
}

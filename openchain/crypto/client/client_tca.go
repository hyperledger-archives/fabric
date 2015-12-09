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

	"bytes"
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

func (client *Client) retrieveTCACertsChain(userId string) error {
	// Retrieve TCA certificate and verify it
	tcaCertRaw, err := client.getTCACertificate()
	if err != nil {
		log.Error("Failed getting TCA certificate %s", err)

		return err
	}
	log.Info("Register:TCAcert %s", utils.EncodeBase64(tcaCertRaw))

	// TODO: Test TCA cert againt root CA
	_, err = utils.DERToX509Certificate(tcaCertRaw)
	if err != nil {
		log.Error("Failed parsing TCA certificate %s", err)

		return err
	}

	// Store TCA cert
	log.Info("Storing TCA certificate for validator [%s]...", userId)

	err = ioutil.WriteFile(getTCACertsChainPath(), utils.DERCertToPEM(tcaCertRaw), 0700)
	if err != nil {
		log.Error("Failed storing tca certificate: %s", err)
		return err
	}

	return nil
}

func (client *Client) storeTCertOwnerKDFKey(pwd []byte) error {
	// TODO: handle synchronization. Multiple instances of Client could be running.

	err := ioutil.WriteFile(getTCertOwnerKDFKeyPath(), utils.AEStoPEM(client.tCertOwnerKDFKey), 0700)
	if err != nil {
		log.Error("Failed storing TCertOwnerKDFKey: %s", err)
		return err
	}

	return nil
}

func (client *Client) loadTCACertsChain() error {
	// Load TCA certs chain
	log.Info("Loading TCA certificates chain at %s...", getTCACertsChainPath())

	chain, err := ioutil.ReadFile(getTCACertsChainPath())
	if err != nil {
		log.Error("Failed loading TCA certificates chain : %s", err.Error())

		return err
	}

	ok := client.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		log.Error("Failed appending TCA certificates chain.")

		return errors.New("Failed appending TCA certificates chain.")
	}

	return nil
}

func (client *Client) loadTCertOwnerKDFKey(pwd []byte) error {
	// Load TCertOwnerKDFKey
	log.Info("Loading TCertOwnerKDFKey at %s...", getTCertOwnerKDFKeyPath())

	missing, _ := utils.FilePathMissing(getTCertOwnerKDFKeyPath())
	if missing {
		log.Info("Loading TCertOwnerKDFKey at %s...done! File is missing.", getTCertOwnerKDFKeyPath())

		return nil
	}

	pem, err := ioutil.ReadFile(getTCertOwnerKDFKeyPath())
	if err != nil {
		log.Error("Failed loading enrollment chain key: %s", err.Error())

		return err
	}

	tCertOwnerKDFKey, err := utils.PEMtoAES(pem, pwd)
	if err != nil {
		log.Error("Failed parsing enrollment chain  key: %s", err.Error())

		return err
	}
	client.tCertOwnerKDFKey = tCertOwnerKDFKey

	log.Info("Loading TCertOwnerKDFKey at %s...done!", getTCertOwnerKDFKeyPath())

	return nil
}

func (client *Client) callTCAReadCertificate(ctx context.Context, in *obcca.TCertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		log.Error("Failed tca dial in: %s", err)

		return nil, err
	}
	defer sockP.Close()

	tcaP := obcca.NewTCAPClient(sockP)

	cert, err := tcaP.ReadCertificate(context.Background(), in)
	if err != nil {
		log.Error("Failed requesting tca read certificate: %s", err)

		return nil, err
	}

	return cert, nil
}

func (client *Client) getTCACertificate() ([]byte, error) {
	log.Info("getTCACertificate...")

	// Prepare the request
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertReadReq{&timestamp, &obcca.Identity{Id: "tca-root"}, nil}
	pbCert, err := client.callTCAReadCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting tca certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert

	log.Info("getTCACertificate...done!")

	return pbCert.Cert, nil
}

// getNextTCert returns the next available (not yet used) transaction certificate
// corresponding to the tuple (cert, signing key)
func (client *Client) getNextTCert() ([]byte, error) {
	log.Info("Getting next TCert...")
	rawCert, err := getDBHandle().GetNextTCert(client.getTCertsFromTCA)
	if err != nil {
		log.Error("getNextTCert: failed accessing db: %s", err)

		return nil, err
	}

	// rawCert and rawKey are supposed to have been already verified at this point.
	log.Info("getNextTCert:cert %s", utils.EncodeBase64(rawCert))

	log.Info("Getting next TCert...done!")

	return rawCert, nil
}

func (client *Client) signWithTCert(tCertDER []byte, msg []byte) ([]byte, error) {
	// Extract the signing key from the tCert

	TCertOwnerEncryptKey := utils.HMACTruncated(client.tCertOwnerKDFKey, []byte{1}, utils.AESKeyLength)
	ExpansionKey := utils.HMAC(client.tCertOwnerKDFKey, []byte{2})

	tCert, err := utils.DERToX509Certificate(tCertDER)
	if err != nil {
		log.Debug("Failed parsing certificate bytes [%s]", utils.EncodeBase64(tCertDER))

		return nil, err
	}

	// 384-bit ExpansionValue = HMAC(Expansion_Key, TCertIndex)
	// Let TCertIndex = Timestamp, RandValue, 1,2,…
	// Timestamp assigned, RandValue assigned and counter reinitialized to 1 per batch

	// TODO: retrieve TCertIndex from the ciphertext encrypted under the TCertOwnerEncryptKey
	ct, err := utils.GetExtension(tCert, utils.TCERT_ENC_TCERTINDEX)
	if err != nil {
		log.Error("Failed getting extension TCERT_ENC_TCERTINDEX: %s", err)

		return nil, err
	}

	// Decrypt ct to TCertIndex (TODO: || EnrollPub_Key || EnrollID ?)
	pt, err := utils.CBCPKCS7Decrypt(TCertOwnerEncryptKey, ct)
	if err != nil {
		log.Error("Failed decrypting extension TCERT_ENC_TCERTINDEX: %s", err)

		return nil, err
	}

	// Compute ExpansionValue based on TCertIndex
	TCertIndex := pt
	//		TCertIndex := []byte(strconv.Itoa(i))

	log.Info("TCertIndex: %s", TCertIndex)
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

	return client.sign(tempSK, msg)
}

func (client *Client) getTCertsFromTCA(num int) ([][]byte, error) {
	log.Info("Get [%d] certificates from the TCA...", num)

	// Contact the TCA
	TCertOwnerKDFKey, derBytes, err := client.tcaCreateCertificateSet(num)
	if err != nil {
		log.Debug("Failed contacting TCA %s", err)

		return nil, err
	}

	if client.tCertOwnerKDFKey != nil {
		// Check that the keys are the same
		equal := bytes.Equal(client.tCertOwnerKDFKey, TCertOwnerKDFKey)
		if !equal {
			return nil, errors.New("Failed reciving kdf key from TCA. The keys are different.")
		}
	} else {
		client.tCertOwnerKDFKey = TCertOwnerKDFKey

		if err := client.storeTCertOwnerKDFKey(nil); err != nil {
			log.Debug("Failed storing TCertOwnerKDFKey: %s", err)
			// TODO: hanlde this situation more carefully
		}
	}

	// TODO: Store TCertOwnerKDFKey and checks that every time it is always the same key

	// Validate the Certificates obtained
	opts := x509.VerifyOptions{
		//		DNSName: "test.example.com",
		Roots: client.rootsCertPool,
	}

	TCertOwnerEncryptKey := utils.HMACTruncated(TCertOwnerKDFKey, []byte{1}, utils.AESKeyLength)
	ExpansionKey := utils.HMAC(TCertOwnerKDFKey, []byte{2})

	resCert := make([][]byte, num)

	j := 0
	for i := 0; i < num; i++ {
		log.Info("Validating certificate. Index [%d]...", i)
		log.Debug("Validating certificate [%s]", utils.EncodeBase64(derBytes[i]))

		certificate, err := utils.DERToX509Certificate(derBytes[i])
		if err != nil {
			log.Debug("Failed parsing certificate bytes [%s]", utils.EncodeBase64(derBytes[i]))

			continue
		}

		// TODO: Verify certificate against root certs
		_, err = certificate.Verify(opts) // TODO: do something with chain of certificate given in output
		if err != nil {
			log.Error("Failed verifing certificate bytes %s", err)

			//			continue
		}

		// Verify public key

		// 384-bit ExpansionValue = HMAC(Expansion_Key, TCertIndex)
		// Let TCertIndex = Timestamp, RandValue, 1,2,…
		// Timestamp assigned, RandValue assigned and counter reinitialized to 1 per batch

		// TODO: retrieve TCertIndex from the ciphertext encrypted under the TCertOwnerEncryptKey
		ct, err := utils.GetExtension(certificate, utils.TCERT_ENC_TCERTINDEX)
		if err != nil {
			log.Error("Failed getting extension TCERT_ENC_TCERTINDEX: %s", err)
			//
			continue
		}

		// Decrypt ct to TCertIndex (TODO: || EnrollPub_Key || EnrollID ?)
		pt, err := utils.CBCPKCS7Decrypt(TCertOwnerEncryptKey, ct)
		if err != nil {
			log.Error("Failed decrypting extension TCERT_ENC_TCERTINDEX: %s", err)

			continue
		}

		// Compute ExpansionValue based on TCertIndex
		TCertIndex := pt
		//		TCertIndex := []byte(strconv.Itoa(i))

		log.Info("TCertIndex: %s", TCertIndex)
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
			log.Error("Failed temporary public key IsOnCurve check.")

			continue
		}

		// Check that the derived public key is the same as the one in the certificate
		certPK := certificate.PublicKey.(*ecdsa.PublicKey)

		cmp := certPK.X.Cmp(tempSK.PublicKey.X)
		if cmp != 0 {
			log.Error("Derived public key is different on X")

			continue
		}

		cmp = certPK.Y.Cmp(tempSK.PublicKey.Y)
		if cmp != 0 {
			log.Error("Derived public key is different on Y")

			continue
		}

		// Verify the signing capability of tempSK
		err = utils.VerifySignCapability(tempSK, certificate.PublicKey)
		if err != nil {
			log.Error("Failed verifing signing capability: %s", err)

			continue
		}

		// Marshall certificate and secret key to be stored in the database
		resCert[j] = derBytes[i]
		if err != nil {
			log.Error("Failed marshalling private key: %s", err)

			continue
		}

		//		log.Debug("key %s", utils.EncodeBase64(resKeys[j]))
		log.Info("Sub index [%d]", j)
		j++
		log.Info("Certificate [%d] validated.", i)
	}

	if j == 0 {
		log.Error("No valid TCert was sent")

		return nil, errors.New("No valid TCert was sent.")
	}

	return resCert[:j], nil
}

func (client *Client) tcaCreateCertificateSet(num int) ([]byte, [][]byte, error) {
	//		return mockTcaCreateCertificates(&client.enrollPrivKey.PublicKey, num)

	sockP, err := grpc.Dial(getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		log.Error("Failed tca dial in: %s", err)

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
		log.Error("Failed marshaling request %s:", err)
		return nil, nil, err
	}

	// 2. Sign rawReq and (TODO) check signature
	log.Info("Signing req %s", utils.EncodeBase64(rawReq))
	r, s, err := client.ecdsaSignWithEnrollmentKey(rawReq)
	if err != nil {
		log.Error("Failed creating signature %s:", err)
		return nil, nil, err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	// 3. Append the signature
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	// 4. Send request
	certSet, err := tcaP.CreateCertificateSet(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting tca create certificate set: %s", err)

		return nil, nil, err
	}

	return certSet.Key, certSet.Certs, nil
}

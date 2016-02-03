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
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"

	"bytes"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/x509"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google/protobuf"
	"math/big"
	"time"
)

func (client *clientImpl) storeTCertOwnerKDFKey() error {
	if err := client.node.ks.storeKey(client.node.conf.getTCertOwnerKDFKeyFilename(), client.tCertOwnerKDFKey); err != nil {
		client.node.log.Error("Failed storing TCertOwnerKDFKey [%s].", err.Error())

		return err
	}
	return nil
}

func (client *clientImpl) loadTCertOwnerKDFKey() error {
	// Load TCertOwnerKDFKey
	client.node.log.Debug("Loading TCertOwnerKDFKey...")

	if !client.node.ks.isAliasSet(client.node.conf.getTCertOwnerKDFKeyFilename()) {
		client.node.log.Debug("Failed loading TCertOwnerKDFKey. Key is missing.")

		return nil
	}

	tCertOwnerKDFKey, err := client.node.ks.loadKey(client.node.conf.getTCertOwnerKDFKeyFilename())
	if err != nil {
		client.node.log.Error("Failed parsing TCertOwnerKDFKey [%s].", err.Error())

		return err
	}
	client.tCertOwnerKDFKey = tCertOwnerKDFKey

	client.node.log.Debug("Loading TCertOwnerKDFKey...done!")

	return nil
}

// getNextTCert returns the next available (not yet used) transaction certificate
// corresponding to the tuple (cert, signing key)
func (client *clientImpl) getNextTCert() ([]byte, error) {
	client.node.log.Debug("Getting next TCert...")
	rawCert, err := client.node.ks.GetNextTCert(client.getTCertsFromTCA)
	if err != nil {
		client.node.log.Error("Failed accessing db [%s].", err.Error())

		return nil, err
	}

	// rawCert and rawKey are supposed to have been already verified at this point.
	client.node.log.Debug("Cert [%s].", utils.EncodeBase64(rawCert))
	//	client.node.log.Info("getNextTCert:key  ", utils.EncodeBase64(rawKey))

	client.node.log.Debug("Getting next TCert...done!")

	return rawCert, nil
}

func (client *clientImpl) signUsingTCertDER(tCertDER []byte, msg []byte) ([]byte, error) {
	// Parse the DER
	tCert, err := utils.DERToX509Certificate(tCertDER)
	if err != nil {
		client.node.log.Error("Failed parsing TCert DER [%s].", err.Error())

		return nil, err
	}

	return client.signUsingTCertX509(tCert, msg)
}

func (client *clientImpl) signUsingTCertX509(tCert *x509.Certificate, msg []byte) ([]byte, error) {
	// Extract the signing key from the tCert
	TCertOwnerEncryptKey := utils.HMACTruncated(client.tCertOwnerKDFKey, []byte{1}, utils.AESKeyLength)
	ExpansionKey := utils.HMAC(client.tCertOwnerKDFKey, []byte{2})

	// TODO: retrieve TCertIndex from the ciphertext encrypted under the TCertOwnerEncryptKey
	ct, err := utils.GetCriticalExtension(tCert, utils.TCertEncTCertIndex)
	if err != nil {
		client.node.log.Error("Failed getting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

		return nil, err
	}

	// Decrypt ct to TCertIndex (TODO: || EnrollPub_Key || EnrollID ?)
	decryptedTCertIndex, err := utils.CBCPKCS7Decrypt(TCertOwnerEncryptKey, ct)
	if err != nil {
		client.node.log.Error("Failed decrypting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

		return nil, err
	}

	// Compute ExpansionValue based on TCertIndex
	TCertIndex := decryptedTCertIndex

	client.node.log.Debug("TCertIndex [%s].", utils.EncodeBase64(TCertIndex))
	mac := hmac.New(utils.NewHash, ExpansionKey)
	mac.Write(TCertIndex)
	ExpansionValue := mac.Sum(nil)

	// Derive tpk and tsk accordingly to ExapansionValue from enrollment pk,sk
	// Computable by TCA / Auditor: TCertPub_Key = EnrollPub_Key + ExpansionValue G
	// using elliptic curve point addition per NIST FIPS PUB 186-4- specified P-384

	// Compute temporary secret key
	tempSK := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: client.node.enrollPrivKey.Curve,
			X:     new(big.Int),
			Y:     new(big.Int),
		},
		D: new(big.Int),
	}

	var k = new(big.Int).SetBytes(ExpansionValue)
	var one = new(big.Int).SetInt64(1)
	n := new(big.Int).Sub(client.node.enrollPrivKey.Params().N, one)
	k.Mod(k, n)
	k.Add(k, one)

	tempSK.D.Add(client.node.enrollPrivKey.D, k)
	tempSK.D.Mod(tempSK.D, client.node.enrollPrivKey.PublicKey.Params().N)

	// Compute temporary public key
	tempX, tempY := client.node.enrollPrivKey.PublicKey.ScalarBaseMult(k.Bytes())
	tempSK.PublicKey.X, tempSK.PublicKey.Y =
		tempSK.PublicKey.Add(
			client.node.enrollPrivKey.PublicKey.X, client.node.enrollPrivKey.PublicKey.Y,
			tempX, tempY,
		)

	return client.node.sign(tempSK, msg)
}

func (client *clientImpl) verifyUsingTCertX509(tCert *x509.Certificate, signature, msg []byte) error {
	ok, err := client.node.verify(tCert.PublicKey, msg, signature)
	if err != nil {
		return err
	}
	if !ok {
		return utils.ErrInvalidSignature
	}
	return nil
}

func (client *clientImpl) getTCertsFromTCA(num int) ([][]byte, error) {
	client.node.log.Debug("Get [%d] certificates from the TCA...", num)

	// Contact the TCA
	TCertOwnerKDFKey, certDERs, err := client.callTCACreateCertificateSet(num)
	if err != nil {
		client.node.log.Debug("Failed contacting TCA [%s].", err.Error())

		return nil, err
	}

	//	client.node.log.Debug("TCertOwnerKDFKey [%s].", utils.EncodeBase64(TCertOwnerKDFKey))

	// Store TCertOwnerKDFKey and checks that every time it is always the same key
	if client.tCertOwnerKDFKey != nil {
		// Check that the keys are the same
		equal := bytes.Equal(client.tCertOwnerKDFKey, TCertOwnerKDFKey)
		if !equal {
			return nil, errors.New("Failed reciving kdf key from TCA. The keys are different.")
		}
	} else {
		client.tCertOwnerKDFKey = TCertOwnerKDFKey

		// TODO: handle this situation more carefully
		if err := client.storeTCertOwnerKDFKey(); err != nil {
			client.node.log.Error("Failed storing TCertOwnerKDFKey [%s].", err.Error())

			return nil, err
		}
	}

	// Validate the Certificates obtained

	TCertOwnerEncryptKey := utils.HMACTruncated(TCertOwnerKDFKey, []byte{1}, utils.AESKeyLength)
	ExpansionKey := utils.HMAC(TCertOwnerKDFKey, []byte{2})

	resCert := make([][]byte, num)

	j := 0
	for i := 0; i < num; i++ {
		client.node.log.Debug("Validating certificate [%d], [%s]", i, utils.EncodeBase64(certDERs[i]))

		// DER to x509
		x509Cert, err := utils.DERToX509Certificate(certDERs[i])
		if err != nil {
			client.node.log.Debug("Failed parsing certificate: [%s].", err)

			continue
		}

		// Handle Critical Extenstion TCertEncTCertIndex
		tCertIndexCT, err := utils.GetCriticalExtension(x509Cert, utils.TCertEncTCertIndex)
		if err != nil {
			client.node.log.Error("Failed getting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

			continue
		}

		// Verify certificate against root
		if _, err := utils.CheckCertAgainRoot(x509Cert, client.node.tcaCertPool); err != nil {
			client.node.log.Warning("Warning verifing certificate [%s].", err.Error())

			continue
		}

		// Verify public key

		// 384-bit ExpansionValue = HMAC(Expansion_Key, TCertIndex)
		// Let TCertIndex = Timestamp, RandValue, 1,2,…
		// Timestamp assigned, RandValue assigned and counter reinitialized to 1 per batch

		// Decrypt ct to TCertIndex (TODO: || EnrollPub_Key || EnrollID ?)
		pt, err := utils.CBCPKCS7Decrypt(TCertOwnerEncryptKey, tCertIndexCT)
		if err != nil {
			client.node.log.Error("Failed decrypting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

			continue
		}

		// Compute ExpansionValue based on TCertIndex
		TCertIndex := pt
		//		TCertIndex := []byte(strconv.Itoa(i))

		client.node.log.Debug("TCertIndex: [%s].", utils.EncodeBase64(TCertIndex))
		mac := hmac.New(utils.NewHash, ExpansionKey)
		mac.Write(TCertIndex)
		ExpansionValue := mac.Sum(nil)

		// Derive tpk and tsk accordingly to ExapansionValue from enrollment pk,sk
		// Computable by TCA / Auditor: TCertPub_Key = EnrollPub_Key + ExpansionValue G
		// using elliptic curve point addition per NIST FIPS PUB 186-4- specified P-384

		// Compute temporary secret key
		tempSK := &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: client.node.enrollPrivKey.Curve,
				X:     new(big.Int),
				Y:     new(big.Int),
			},
			D: new(big.Int),
		}

		var k = new(big.Int).SetBytes(ExpansionValue)
		var one = new(big.Int).SetInt64(1)
		n := new(big.Int).Sub(client.node.enrollPrivKey.Params().N, one)
		k.Mod(k, n)
		k.Add(k, one)

		tempSK.D.Add(client.node.enrollPrivKey.D, k)
		tempSK.D.Mod(tempSK.D, client.node.enrollPrivKey.PublicKey.Params().N)

		// Compute temporary public key
		tempX, tempY := client.node.enrollPrivKey.PublicKey.ScalarBaseMult(k.Bytes())
		tempSK.PublicKey.X, tempSK.PublicKey.Y =
			tempSK.PublicKey.Add(
				client.node.enrollPrivKey.PublicKey.X, client.node.enrollPrivKey.PublicKey.Y,
				tempX, tempY,
			)

		// Verify temporary public key is a valid point on the reference curve
		isOn := tempSK.Curve.IsOnCurve(tempSK.PublicKey.X, tempSK.PublicKey.Y)
		if !isOn {
			client.node.log.Error("Failed temporary public key IsOnCurve check.")

			continue
		}

		// Check that the derived public key is the same as the one in the certificate
		certPK := x509Cert.PublicKey.(*ecdsa.PublicKey)

		cmp := certPK.X.Cmp(tempSK.PublicKey.X)
		if cmp != 0 {
			client.node.log.Error("Derived public key is different on X")

			continue
		}

		cmp = certPK.Y.Cmp(tempSK.PublicKey.Y)
		if cmp != 0 {
			client.node.log.Error("Derived public key is different on Y")

			continue
		}

		// Verify the signing capability of tempSK
		err = utils.VerifySignCapability(tempSK, x509Cert.PublicKey)
		if err != nil {
			client.node.log.Error("Failed verifing signing capability [%s].", err.Error())

			continue
		}

		// Marshall certificate and secret key to be stored in the database
		resCert[j] = certDERs[i]
		if err != nil {
			client.node.log.Error("Failed marshalling private key [%s].", err.Error())

			continue
		}

		if err := utils.CheckCertPKAgainstSK(x509Cert, interface{}(tempSK)); err != nil {
			client.node.log.Error("Failed checking TCA cert PK against private key [%s].", err.Error())

			continue
		}

		client.node.log.Debug("Sub index [%d]", j)
		j++
		client.node.log.Debug("Certificate [%d] validated.", i)
	}

	if j == 0 {
		client.node.log.Error("No valid TCert was sent")

		return nil, errors.New("No valid TCert was sent.")
	}

	return resCert[:j], nil
}

func (client *clientImpl) callTCACreateCertificateSet(num int) ([]byte, [][]byte, error) {
	// Get a TCA Client
	sock, tcaP, err := client.node.getTCAClient()
	defer sock.Close()

	// Execute the protocol
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertCreateSetReq{
		&timestamp,
		&obcca.Identity{Id: client.node.enrollID},
		uint32(num),
		nil,
	}
	rawReq, err := proto.Marshal(req)
	if err != nil {
		client.node.log.Error("Failed marshaling request [%s] [%s].", err.Error())
		return nil, nil, err
	}

	// 2. Sign rawReq
	client.node.log.Debug("Signing req [%s]", utils.EncodeBase64(rawReq))
	r, s, err := client.node.ecdsaSignWithEnrollmentKey(rawReq)
	if err != nil {
		client.node.log.Error("Failed creating signature [%s] [%s].", err.Error())
		return nil, nil, err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	// 3. Append the signature
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	// 4. Send request
	certSet, err := tcaP.CreateCertificateSet(context.Background(), req)
	if err != nil {
		client.node.log.Error("Failed requesting tca create certificate set [%s].", err.Error())

		return nil, nil, err
	}

	return certSet.Certs.Key, certSet.Certs.Certs, nil
}

func (client *clientImpl) validateTCert(tCertDER []byte) (*x509.Certificate, error) {
	client.node.log.Debug("Validating TCert [%s]", utils.EncodeBase64(tCertDER))

	// DER to x509
	x509Cert, err := utils.DERToX509Certificate(tCertDER)
	if err != nil {
		client.node.log.Debug("Failed parsing certificate: [%s].", err)

		return nil, err
	}

	// Handle Critical Extension TCertEncTCertIndex
	if _, err = utils.GetCriticalExtension(x509Cert, utils.TCertEncTCertIndex); err != nil {
		client.node.log.Error("Failed getting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

		return nil, err
	}

	// Verify certificate against root
	if _, err := utils.CheckCertAgainRoot(x509Cert, client.node.tcaCertPool); err != nil {
		client.node.log.Warning("Warning verifing certificate [%s].", err.Error())

		return nil, err
	}

	return x509Cert, nil
}

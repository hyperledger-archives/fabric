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
	obcca "github.com/hyperledger/fabric/membersrvc/protos"

	"bytes"
	"crypto/ecdsa"
	"crypto/hmac"
	"encoding/asn1"

	"strings"
	"strconv"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"golang.org/x/net/context"
	"google/protobuf"
	"math/big"
	"time"
)

func (client *clientImpl) initTCertEngine() (err error) {
	// load TCertOwnerKDFKey
	if err = client.loadTCertOwnerKDFKey(); err != nil {
		return
	}

	// init TCerPool
	client.debug("Using multithreading [%t]", client.conf.IsMultithreadingEnabled())
	client.debug("TCert batch size [%d]", client.conf.getTCertBatchSize())

	if client.conf.IsMultithreadingEnabled() {
		client.tCertPool = new(tCertPoolMultithreadingImpl)
	} else {
		client.tCertPool = new(tCertPoolSingleThreadImpl)
	}

	if err = client.tCertPool.init(client); err != nil {
		client.error("Failied inizializing TCertPool: [%s]", err)

		return
	}
	if err = client.tCertPool.Start(); err != nil {
		client.error("Failied starting TCertPool: [%s]", err)

		return
	}
	return
}

func (client *clientImpl) storeTCertOwnerKDFKey() error {
	if err := client.ks.storeKey(client.conf.getTCertOwnerKDFKeyFilename(), client.tCertOwnerKDFKey); err != nil {
		client.error("Failed storing TCertOwnerKDFKey [%s].", err.Error())

		return err
	}
	return nil
}

func (client *clientImpl) loadTCertOwnerKDFKey() error {
	// Load TCertOwnerKDFKey
	client.debug("Loading TCertOwnerKDFKey...")

	if !client.ks.isAliasSet(client.conf.getTCertOwnerKDFKeyFilename()) {
		client.debug("Failed loading TCertOwnerKDFKey. Key is missing.")

		return nil
	}

	tCertOwnerKDFKey, err := client.ks.loadKey(client.conf.getTCertOwnerKDFKeyFilename())
	if err != nil {
		client.error("Failed parsing TCertOwnerKDFKey [%s].", err.Error())

		return err
	}
	client.tCertOwnerKDFKey = tCertOwnerKDFKey

	client.debug("Loading TCertOwnerKDFKey...done!")

	return nil
}

func (client *clientImpl) getTCertFromExternalDER(der []byte) (TransactionCertificate, error) {
	return client.getTCertFromDER(der, false)
}

func (client *clientImpl) getTCertFromDER(der []byte, failOnLoadSK bool) (tCert TransactionCertificate, err error) {
	if client.tCertOwnerKDFKey == nil {
		return nil, fmt.Errorf("KDF key not initialized yet")
	}

	TCertOwnerEncryptKey := primitives.HMACTruncated(client.tCertOwnerKDFKey, []byte{1}, primitives.AESKeyLength)
	ExpansionKey := primitives.HMAC(client.tCertOwnerKDFKey, []byte{2})

	// DER to x509
	x509Cert, err := primitives.DERToX509Certificate(der)
	if err != nil {
		client.debug("Failed parsing certificate [% x]: [%s].", der, err)

		return
	}

	// Handle Critical Extenstion TCertEncTCertIndex
	tCertIndexCT, err := primitives.GetCriticalExtension(x509Cert, primitives.TCertEncTCertIndex)
	if err != nil {
		client.error("Failed getting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

		return
	}

	// Verify certificate against root
	if _, err = primitives.CheckCertAgainRoot(x509Cert, client.tcaCertPool); err != nil {
		client.warning("Warning verifing certificate [%s].", err.Error())

		return
	}

	tCert = &transactionCertificateImpl{client, x509Cert, nil}

	// Verify public key

	// 384-bit ExpansionValue = HMAC(Expansion_Key, TCertIndex)
	// Let TCertIndex = Timestamp, RandValue, 1,2,…
	// Timestamp assigned, RandValue assigned and counter reinitialized to 1 per batch

	// Decrypt ct to TCertIndex (TODO: || EnrollPub_Key || EnrollID ?)
	pt, err := primitives.CBCPKCS7Decrypt(TCertOwnerEncryptKey, tCertIndexCT)
	if err != nil {
		client.error("Failed decrypting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

		if failOnLoadSK {
			return
		} else {
			return tCert, nil
		}
	}

	// Compute ExpansionValue based on TCertIndex
	TCertIndex := pt
	//		TCertIndex := []byte(strconv.Itoa(i))

	client.debug("TCertIndex: [% x].", TCertIndex)
	mac := hmac.New(primitives.NewHash, ExpansionKey)
	mac.Write(TCertIndex)
	ExpansionValue := mac.Sum(nil)

	// Derive tpk and tsk accordingly to ExapansionValue from enrollment pk,sk
	// Computable by TCA / Auditor: TCertPub_Key = EnrollPub_Key + ExpansionValue G
	// using elliptic curve point addition per NIST FIPS PUB 186-4- specified P-384

	// Compute temporary secret key
	tempSK := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: client.enrollSigningKey.Curve,
			X:     new(big.Int),
			Y:     new(big.Int),
		},
		D: new(big.Int),
	}

	var k = new(big.Int).SetBytes(ExpansionValue)
	var one = new(big.Int).SetInt64(1)
	n := new(big.Int).Sub(client.enrollSigningKey.Params().N, one)
	k.Mod(k, n)
	k.Add(k, one)

	tempSK.D.Add(client.enrollSigningKey.D, k)
	tempSK.D.Mod(tempSK.D, client.enrollSigningKey.PublicKey.Params().N)

	// Compute temporary public key
	tempX, tempY := client.enrollSigningKey.PublicKey.ScalarBaseMult(k.Bytes())
	tempSK.PublicKey.X, tempSK.PublicKey.Y =
		tempSK.PublicKey.Add(
			client.enrollSigningKey.PublicKey.X, client.enrollSigningKey.PublicKey.Y,
			tempX, tempY,
		)

	// Verify temporary public key is a valid point on the reference curve
	isOn := tempSK.Curve.IsOnCurve(tempSK.PublicKey.X, tempSK.PublicKey.Y)
	if !isOn {
		client.error("Failed temporary public key IsOnCurve check.")

		if failOnLoadSK {
			return nil, fmt.Errorf("Failed temporary public key IsOnCurve check.")
		} else {
			return tCert, nil
		}
	}

	// Check that the derived public key is the same as the one in the certificate
	certPK := x509Cert.PublicKey.(*ecdsa.PublicKey)

	if certPK.X.Cmp(tempSK.PublicKey.X) != 0 {
		client.error("Derived public key is different on X")

		if failOnLoadSK {
			return nil, fmt.Errorf("Derived public key is different on X")
		} else {
			return tCert, nil
		}
	}

	if certPK.Y.Cmp(tempSK.PublicKey.Y) != 0 {
		client.error("Derived public key is different on Y")

		if failOnLoadSK {
			return nil, fmt.Errorf("Derived public key is different on Y")
		} else {
			return tCert, nil
		}
	}

	// Verify the signing capability of tempSK
	err = primitives.VerifySignCapability(tempSK, x509Cert.PublicKey)
	if err != nil {
		client.error("Failed verifing signing capability [%s].", err.Error())

		if failOnLoadSK {
			return
		} else {
			return tCert, nil
		}
	}

	// Marshall certificate and secret key to be stored in the database
	if err != nil {
		client.error("Failed marshalling private key [%s].", err.Error())

		if failOnLoadSK {
			return
		} else {
			return tCert, nil
		}
	}

	if err = primitives.CheckCertPKAgainstSK(x509Cert, interface{}(tempSK)); err != nil {
		client.error("Failed checking TCA cert PK against private key [%s].", err.Error())

		if failOnLoadSK {
			return
		} else {
			return tCert, nil
		}
	}

	tCert = &transactionCertificateImpl{client, x509Cert, tempSK}

	return
}

func (client *clientImpl) getTCertsFromTCA(num int) error {
	client.debug("Get [%d] certificates from the TCA...", num)

	// Contact the TCA
	TCertOwnerKDFKey, certDERs, err := client.callTCACreateCertificateSet(num)
	if err != nil {
		client.debug("Failed contacting TCA [%s].", err.Error())

		return err
	}

	//	client.debug("TCertOwnerKDFKey [%s].", utils.EncodeBase64(TCertOwnerKDFKey))

	// Store TCertOwnerKDFKey and checks that every time it is always the same key
	if client.tCertOwnerKDFKey != nil {
		// Check that the keys are the same
		equal := bytes.Equal(client.tCertOwnerKDFKey, TCertOwnerKDFKey)
		if !equal {
			return errors.New("Failed reciving kdf key from TCA. The keys are different.")
		}
	} else {
		client.tCertOwnerKDFKey = TCertOwnerKDFKey

		// TODO: handle this situation more carefully
		if err := client.storeTCertOwnerKDFKey(); err != nil {
			client.error("Failed storing TCertOwnerKDFKey [%s].", err.Error())

			return err
		}
	}

	// Validate the Certificates obtained

	TCertOwnerEncryptKey := primitives.HMACTruncated(client.tCertOwnerKDFKey, []byte{1}, primitives.AESKeyLength)
	ExpansionKey := primitives.HMAC(client.tCertOwnerKDFKey, []byte{2})

	j := 0
	for i := 0; i < num; i++ {
		// DER to x509
		x509Cert, err := primitives.DERToX509Certificate(certDERs[i].Cert)
		if err != nil {
			client.debug("Failed parsing certificate [% x]: [%s].", certDERs[i].Cert, err)

			continue
		}

		// Handle Critical Extenstion TCertEncTCertIndex
		tCertIndexCT, err := primitives.GetCriticalExtension(x509Cert, primitives.TCertEncTCertIndex)
		if err != nil {
			client.error("Failed getting extension TCERT_ENC_TCERTINDEX [% x]: [%s].", err)

			continue
		}

		// Verify certificate against root
		if _, err := primitives.CheckCertAgainRoot(x509Cert, client.tcaCertPool); err != nil {
			client.warning("Warning verifing certificate [%s].", err.Error())

			continue
		}

		// Verify public key

		// 384-bit ExpansionValue = HMAC(Expansion_Key, TCertIndex)
		// Let TCertIndex = Timestamp, RandValue, 1,2,…
		// Timestamp assigned, RandValue assigned and counter reinitialized to 1 per batch

		// Decrypt ct to TCertIndex (TODO: || EnrollPub_Key || EnrollID ?)
		pt, err := primitives.CBCPKCS7Decrypt(TCertOwnerEncryptKey, tCertIndexCT)
		if err != nil {
			client.error("Failed decrypting extension TCERT_ENC_TCERTINDEX [%s].", err.Error())

			continue
		}

		// Compute ExpansionValue based on TCertIndex
		TCertIndex := pt
		//		TCertIndex := []byte(strconv.Itoa(i))

		client.debug("TCertIndex: [% x].", TCertIndex)
		mac := hmac.New(primitives.NewHash, ExpansionKey)
		mac.Write(TCertIndex)
		ExpansionValue := mac.Sum(nil)

		// Derive tpk and tsk accordingly to ExapansionValue from enrollment pk,sk
		// Computable by TCA / Auditor: TCertPub_Key = EnrollPub_Key + ExpansionValue G
		// using elliptic curve point addition per NIST FIPS PUB 186-4- specified P-384

		// Compute temporary secret key
		tempSK := &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: client.enrollSigningKey.Curve,
				X:     new(big.Int),
				Y:     new(big.Int),
			},
			D: new(big.Int),
		}

		var k = new(big.Int).SetBytes(ExpansionValue)
		var one = new(big.Int).SetInt64(1)
		n := new(big.Int).Sub(client.enrollSigningKey.Params().N, one)
		k.Mod(k, n)
		k.Add(k, one)

		tempSK.D.Add(client.enrollSigningKey.D, k)
		tempSK.D.Mod(tempSK.D, client.enrollSigningKey.PublicKey.Params().N)

		// Compute temporary public key
		tempX, tempY := client.enrollSigningKey.PublicKey.ScalarBaseMult(k.Bytes())
		tempSK.PublicKey.X, tempSK.PublicKey.Y =
			tempSK.PublicKey.Add(
				client.enrollSigningKey.PublicKey.X, client.enrollSigningKey.PublicKey.Y,
				tempX, tempY,
			)

		// Verify temporary public key is a valid point on the reference curve
		isOn := tempSK.Curve.IsOnCurve(tempSK.PublicKey.X, tempSK.PublicKey.Y)
		if !isOn {
			client.error("Failed temporary public key IsOnCurve check.")

			continue
		}

		// Check that the derived public key is the same as the one in the certificate
		certPK := x509Cert.PublicKey.(*ecdsa.PublicKey)

		if certPK.X.Cmp(tempSK.PublicKey.X) != 0 {
			client.error("Derived public key is different on X")

			continue
		}

		if certPK.Y.Cmp(tempSK.PublicKey.Y) != 0 {
			client.error("Derived public key is different on Y")

			continue
		}

		// Verify the signing capability of tempSK
		err = primitives.VerifySignCapability(tempSK, x509Cert.PublicKey)
		if err != nil {
			client.error("Failed verifing signing capability [%s].", err.Error())

			continue
		}

		// Marshall certificate and secret key to be stored in the database
		if err != nil {
			client.error("Failed marshalling private key [%s].", err.Error())

			continue
		}

		if err := primitives.CheckCertPKAgainstSK(x509Cert, interface{}(tempSK)); err != nil {
			client.error("Failed checking TCA cert PK against private key [%s].", err.Error())

			continue
		}

		client.debug("Sub index [%d]", j)
		j++
		client.debug("Certificate [%d] validated.", i)

		client.tCertPool.AddTCert(&transactionCertificateImpl{client, x509Cert, tempSK})
	}

	if j == 0 {
		client.error("No valid TCert was sent")

		return errors.New("No valid TCert was sent.")
	}

	return nil
}

func (client *clientImpl) callTCACreateCertificateSet(num int) ([]byte, []*membersrvc.TCert, error) {
	// Get a TCA Client
	sock, tcaP, err := client.getTCAClient()
	defer sock.Close()

	// Execute the protocol
	now := time.Now()
	timestamp := google_protobuf.Timestamp{Seconds: int64(now.Second()), Nanos: int32(now.Nanosecond())}
	req := &obcca.TCertCreateSetReq{
		Ts:  &timestamp,
		Id:  &obcca.Identity{Id: client.enrollID},
		Num: uint32(num),
		Attributes: client.conf.getTCertAttributes(),
		Sig: nil,
	}
	
	rawReq, err := proto.Marshal(req)
	if err != nil {
		client.error("Failed marshaling request [%s] [%s].", err.Error())
		return nil, nil, err
	}

	// 2. Sign rawReq
	r, s, err := client.ecdsaSignWithEnrollmentKey(rawReq)
	if err != nil {
		client.error("Failed creating signature for [% x]: [%s].", rawReq, err.Error())
		return nil, nil, err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	// 3. Append the signature
	req.Sig = &obcca.Signature{Type: obcca.CryptoType_ECDSA, R: R, S: S}

	// 4. Send request
	certSet, err := tcaP.CreateCertificateSet(context.Background(), req)
	if err != nil {
		client.error("Failed requesting tca create certificate set [%s].", err.Error())

		return nil, nil, err
	}

	return certSet.Certs.Key, certSet.Certs.Certs, nil
}

func (client *clientImpl) parseHeader(header string) (map[string]int, error) { 
	tokens :=  strings.Split(header, "#")
	answer := make(map[string]int)
	
	for _, token := range tokens {
		pair:= strings.Split(token, "->")
		
		if len(pair) == 2 {
			key := pair[0]
			valueStr := pair[1]
			value, err := strconv.Atoi(valueStr)
			if err != nil { 
				return nil, err
			}
			answer[key] = value
		}
	}
	
	return answer, nil
	
}
// Read the attribute with name 'attributeName' from the der encoded x509.Certificate 'tcertder'.
func (client *clientImpl) ReadAttribute(attributeName string, tcertder []byte) ([]byte, error) {
	tcert, err := utils.DERToX509Certificate(tcertder)
	if err != nil {
		client.debug("Failed parsing certificate [% x]: [%s].", tcertder, err)

		return nil, err
	}
	
	var header_raw []byte
	if header_raw, err = utils.GetCriticalExtension(tcert, utils.TCertAttributesHeaders); err != nil {
		client.error("Failed getting extension TCERT_ATTRIBUTES_HEADER [% x]: [%s].", tcertder, err)

		return nil, err
	}
	
	header_str := string(header_raw)	
	var header map[string]int
	header, err = client.parseHeader(header_str)
	
	if err != nil {
		return nil, err
	}
	
	position := header[attributeName]
	
	if position == 0 {
		return nil, errors.New("Failed attribute doesn't exists in the TCert.")
	}

    oid := asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 9 + position}
    
    var value []byte
    if value, err = utils.GetCriticalExtension(tcert, oid); err != nil {
		client.error("Failed getting extension Attribute Value [% x]: [%s].", tcertder, err)
		return nil, err
	}
    
    return value, nil
}

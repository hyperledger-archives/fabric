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

package obcca

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"math"
	"math/big"
	"strconv"

	protobuf "google/protobuf"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"
	"github.com/openblockchain/obc-peer/openchain/crypto/conf"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	// TCertEncTCertIndex is the ASN1 object identifier of the TCert index.
	//
	TCertEncTCertIndex = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 7}
	TCertEncEnrollmentID = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 8}
)

// TCA is the transaction certificate authority.
//
type TCA struct {
	*CA
	eca     *ECA
	hmacKey []byte
}

// TCAP serves the public GRPC interface of the TCA.
//
type TCAP struct {
	tca *TCA
}

// TCAA serves the administrator GRPC interface of the TCA.
//
type TCAA struct {
	tca *TCA
}

// NewTCA sets up a new TCA.
//
func NewTCA(eca *ECA) *TCA {
	var cooked string

	tca := &TCA{NewCA("tca"), eca, nil}

	raw, err := ioutil.ReadFile(tca.path + "/tca.hmac")
	if err != nil {
		key := make([]byte, 49)
		rand.Reader.Read(key)
		cooked = base64.StdEncoding.EncodeToString(key)

		err = ioutil.WriteFile(tca.path+"/tca.hmac", []byte(cooked), 0644)
		if err != nil {
			Panic.Panicln(err)
		}
	} else {
		cooked = string(raw)
	}

	tca.hmacKey, err = base64.StdEncoding.DecodeString(cooked)
	if err != nil {
		Panic.Panicln(err)
	}

	return tca
}

// Start starts the TCA.
//
func (tca *TCA) Start(srv *grpc.Server) {
	tca.startTCAP(srv)
	tca.startTCAA(srv)

	tca.startValidityPeriodUpdate()
	Info.Println("TCA started.")
}

func (tca *TCA) startValidityPeriodUpdate() {
	if validityPeriodUpdateEnabled() {
		go updateValidityPeriod()
	}
}

func (tca *TCA) startTCAP(srv *grpc.Server) {
	pb.RegisterTCAPServer(srv, &TCAP{tca})
}

func (tca *TCA) startTCAA(srv *grpc.Server) {
	pb.RegisterTCAAServer(srv, &TCAA{tca})
}

// ReadCACertificate reads the certificate of the TCA.
//
func (tcap *TCAP) ReadCACertificate(ctx context.Context, in *pb.Empty) (*pb.Cert, error) {
	Trace.Println("grpc TCAP:ReadCACertificate")

	return &pb.Cert{tcap.tca.raw}, nil
}

// CreateCertificate requests the creation of a new transaction certificate by the TCA.
//
func (tcap *TCAP) CreateCertificate(ctx context.Context, in *pb.TCertCreateReq) (*pb.TCertCreateResp, error) {
	Trace.Println("grpc TCAP:CreateCertificate")

	id := in.Id.Id
	raw, err := tcap.tca.eca.readCertificate(id, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}

	sig := in.Sig
	in.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	raw = in.Pub.Key
	if in.Pub.Type != pb.CryptoType_ECDSA {
		return nil, errors.New("unsupported key type")
	}
	pub, err := x509.ParsePKIXPublicKey(in.Pub.Key)
	if err != nil {
		return nil, err
	}

	hash := utils.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	if raw, err = tcap.tca.createCertificate(id, pub.(*ecdsa.PublicKey), x509.KeyUsageDigitalSignature, in.Ts.Seconds, nil); err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.TCertCreateResp{&pb.Cert{raw}}, nil
}

func getPreKFrom(enrollmentCert *x509.Certificate) ([]byte, error) {
	//Implement the code that returns an raw corresponding to the PreK of this ECert.
	key := make([]byte, 40) 
	rand.Reader.Read(key)
	return key, nil
}
// CreateCertificateSet requests the creation of a new transaction certificate set by the TCA.
//
func (tcap *TCAP) CreateCertificateSet(ctx context.Context, in *pb.TCertCreateSetReq) (*pb.TCertCreateSetResp, error) {
	Trace.Println("grpc TCAP:CreateCertificateSet")

	id := in.Id.Id
	raw, err := tcap.tca.eca.readCertificate(id, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	pub := cert.PublicKey.(*ecdsa.PublicKey)

	sig := in.Sig
	in.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	hash := utils.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(pub, hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	nonce := make([]byte, 16) // 8 bytes rand, 8 bytes timestamp
	rand.Reader.Read(nonce[:8])
	binary.LittleEndian.PutUint64(nonce[8:], uint64(in.Ts.Seconds))

	mac := hmac.New(conf.GetDefaultHash(), tcap.tca.hmacKey)
	raw, _ = x509.MarshalPKIXPublicKey(pub)
	mac.Write(raw)
	kdfKey := mac.Sum(nil)
	
	num := int(in.Num)
	if num == 0 {
		num = 1
	}
	
	// the batch of TCerts
	var set [][]byte

	for i := 0; i < num; i++ {
		tidx := []byte(strconv.Itoa(2*i + 1))
		tidx = append(tidx[:], nonce[:]...)
		
		tcertid := util.GenerateIntUUID()
		
		// TODO: We are storing each K used on the TCert in the ks array (the second return value of this call), but not returning it to the user.
		// We need to design a structure to return each TCert and the associated Ks.
		extensions, _, err := generateEncryptedExtensions(id, tcertid, cert, in.Attributes, kdfKey, tidx)
		if err != nil {
			return nil, err
		}
		
		one := new(big.Int).SetInt64(1)
		k := new(big.Int).SetBytes(mac.Sum(nil))
		k.Mod(k, new(big.Int).Sub(pub.Curve.Params().N, one))
		k.Add(k, one)
		
		tmpX, tmpY := pub.ScalarBaseMult(k.Bytes())
		txX, txY := pub.Curve.Add(pub.X, pub.Y, tmpX, tmpY)
		txPub := ecdsa.PublicKey{Curve: pub.Curve, X: txX, Y: txY}
		
		spec := NewDefaultPeriodCertificateSpec(id, tcertid, &txPub,  x509.KeyUsageDigitalSignature, extensions...)
		if raw, err = tcap.tca.createCertificateFromSpec(spec, in.Ts.Seconds, kdfKey); err != nil {
			Error.Println(err)
			return nil, err
		}
		set = append(set, raw)
	}

	return &pb.TCertCreateSetResp{&pb.CertSet{in.Ts, in.Id, kdfKey, set}}, nil
}

func generateEncryptedExtensions(id string, tcertid *big.Int, enrollmentCert *x509.Certificate, attributes map[string]string, kdfKey []byte, tidx []byte) ([]pkix.Extension, [][]byte, error){
	// For each TCert we need to store and retrieve to the user the list of Ks used to encrypt the EnrollmentID and the attributes.
	var ks [][]byte
	
	// Generate the extensions for the current TCert. Extensions contain:
	// - encrypted TCertIndex
	// - encrypted EnrollmentID
	// - encrypted attributes
	extensions := make([]pkix.Extension, len(attributes))

	// Compute encrypted TCertIndex
	mac := hmac.New(conf.GetDefaultHash(), kdfKey)
	mac.Write([]byte{1})
	extKey := mac.Sum(nil)[:32]
	
	mac = hmac.New(conf.GetDefaultHash(), kdfKey)
	mac.Write([]byte{2})
	mac = hmac.New(conf.GetDefaultHash(), mac.Sum(nil))
	mac.Write(tidx)
	
	ext, err := CBCEncrypt(extKey, tidx)
	if err != nil {
		return nil, nil, err
	}
	
	// Append the TCertIndex to the extensions
	extensions = append(extensions, pkix.Extension{Id: TCertEncTCertIndex, Critical: true, Value: ext})

	// TODO: Check if we need to do the same logic as the one used for attribute encryption (compute preK_1 and then preK_0 using the string "enrollmentID").
	var preKey []byte
	preKey, err  = getPreKFrom(enrollmentCert)
	
	// Compute encrypted EnrollmentID
	mac = hmac.New(conf.GetDefaultHash(), preKey)
	mac.Write(tcertid.Bytes())
	
	enrollmentIdKey := mac.Sum(nil)
	
	//TODO: we need to check if it is better to use EBC encryption, it is not implemented right now in the project.
	encEnrollmentID, err := CBCEncrypt(enrollmentIdKey, []byte(id))
	if err != nil {
		return nil, nil, err
	}
	
	// Append the encrypted EnrollmentID to the extensions
	extensions = append(extensions, pkix.Extension{Id: TCertEncEnrollmentID, Critical: true, Value: encEnrollmentID})
	
	// save k used to encrypt EnrollmentID
	ks = append(ks, enrollmentIdKey)
	
	// Compute preK_1 to encrypt attributes - TODO compute preK_1 properly based on user's affiliation
	preK_1, err := getPreKFrom(enrollmentCert)
	if err != nil {
		return nil, nil, err
	}
	
	// Encrypt and append attributes to the extensions slice
	for attributeName, attributeValue := range attributes {
		mac = hmac.New(conf.GetDefaultHash(), preK_1)
		mac.Write(tcertid.Bytes())
		preK_0 := mac.Sum(nil)
		
		mac = hmac.New(conf.GetDefaultHash(), preK_0)
		mac.Write([]byte(attributeName))
		attributeKey := mac.Sum(nil)
		
		// TODO: should we encrypt the value of the attribute along with the attribute name? Something like enc(attributeName:attributeValue).
		// Also, we need to check if it is better to use EBC encryption, it is not implemented right now in the project.
		encryptedAttributed, err := CBCEncrypt(attributeKey, []byte(attributeValue))
		if err != nil {
			return nil, nil, err
		}
		
		// Append the encrypted attribute to the extensions
		extensions = append(extensions, pkix.Extension{Id: TCertEncEnrollmentID, Critical: true, Value: encryptedAttributed})
		
		// save k used to encrypt attribute
		ks = append(ks, attributeKey)
	}
	
	return extensions, ks, nil
}

// ReadCertificate reads a transaction certificate from the TCA.
//
func (tcap *TCAP) ReadCertificate(ctx context.Context, in *pb.TCertReadReq) (*pb.Cert, error) {
	Trace.Println("grpc TCAP:ReadCertificate")

	req := in.Req.Id
	id := in.Id.Id

	if req != id && tcap.tca.eca.readRole(req)&(int(pb.Role_VALIDATOR)|int(pb.Role_AUDITOR)) == 0 {
		return nil, errors.New("access denied")
	}

	raw, err := tcap.tca.eca.readCertificate(req, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}

	sig := in.Sig
	in.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	hash := utils.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	if in.Ts.Seconds != 0 {
		raw, err = tcap.tca.readCertificate1(id, in.Ts.Seconds)
	} else {
		raw, err = tcap.tca.readCertificateByHash(in.Hash.Hash)
	}
	if err != nil {
		return nil, err
	}

	return &pb.Cert{raw}, nil
}

// ReadCertificateSet reads a transaction certificate set from the TCA.  Not yet implemented.
//
func (tcap *TCAP) ReadCertificateSet(ctx context.Context, in *pb.TCertReadSetReq) (*pb.CertSet, error) {
	Trace.Println("grpc TCAP:ReadCertificateSet")

	req := in.Req.Id
	id := in.Id.Id

	if req != id && tcap.tca.eca.readRole(req)&int(pb.Role_AUDITOR) == 0 {
		return nil, errors.New("access denied")
	}

	raw, err := tcap.tca.eca.readCertificate(req, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}

	sig := in.Sig
	in.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	hash := utils.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	rows, err := tcap.tca.readCertificates(id, in.Ts.Seconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs [][]byte
	var kdfKey []byte
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw, &kdfKey); err != nil {
			return nil, err
		}

		certs = append(certs, raw)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &pb.CertSet{in.Ts, in.Id, kdfKey, certs}, nil
}

// RevokeCertificate revokes a certificate from the TCA.  Not yet implemented.
//
func (tcap *TCAP) RevokeCertificate(context.Context, *pb.TCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificateSet revokes a certificate set from the TCA.  Not yet implemented.
//
func (tcap *TCAP) RevokeCertificateSet(context.Context, *pb.TCertRevokeSetReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAP:RevokeCertificateSet")

	return nil, errors.New("not yet implemented")
}

// ReadCertificateSets returns all certificates matching the filter criteria of the request.
//
func (tcaa *TCAA) ReadCertificateSets(ctx context.Context, in *pb.TCertReadSetsReq) (*pb.CertSets, error) {
	Trace.Println("grpc TCAA:ReadCertificateSets")

	req := in.Req.Id
	if tcaa.tca.eca.readRole(req)&int(pb.Role_AUDITOR) == 0 {
		return nil, errors.New("access denied")
	}

	raw, err := tcaa.tca.eca.readCertificate(req, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}

	sig := in.Sig
	in.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	hash := utils.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	users, err := tcaa.tca.eca.readUsers(int(in.Role))
	if err != nil {
		return nil, err
	}
	defer users.Close()

	begin := int64(0)
	end := int64(math.MaxInt64)
	if in.Begin != nil {
		begin = in.Begin.Seconds
	}
	if in.End != nil {
		end = in.End.Seconds
	}

	var sets []*pb.CertSet
	for users.Next() {
		var id string
		var role int
		if err = users.Scan(&id, &role); err != nil {
			return nil, err
		}

		rows, err := tcaa.tca.eca.readCertificateSets(id, begin, end)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var certs [][]byte
		var kdfKey []byte
		var timestamp int64
		timestamp = 0

		for rows.Next() {
			var cert []byte
			var ts int64

			if err = rows.Scan(&cert, &kdfKey, &ts); err != nil {
				return nil, err
			}

			if ts != timestamp {
				sets = append(sets, &pb.CertSet{&protobuf.Timestamp{Seconds: timestamp, Nanos: 0}, &pb.Identity{id}, kdfKey, certs})

				timestamp = ts
				certs = nil
			}

			certs = append(certs, cert)
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}

		sets = append(sets, &pb.CertSet{&protobuf.Timestamp{Seconds: timestamp, Nanos: 0}, &pb.Identity{id}, kdfKey, certs})
	}
	if err = users.Err(); err != nil {
		return nil, err
	}

	return &pb.CertSets{sets}, nil
}

// RevokeCertificate revokes a certificate from the TCA.  Not yet implemented.
//
func (tcaa *TCAA) RevokeCertificate(context.Context, *pb.TCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificateSet revokes a certificate set from the TCA.  Not yet implemented.
//
func (tcaa *TCAA) RevokeCertificateSet(context.Context, *pb.TCertRevokeSetReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:RevokeCertificateSet")

	return nil, errors.New("not yet implemented")
}

// PublishCRL requests the creation of a certificate revocation list from the TCA.  Not yet implemented.
//
func (tcaa *TCAA) PublishCRL(context.Context, *pb.TCertCRLReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:CreateCRL")

	return nil, errors.New("not yet implemented")
}

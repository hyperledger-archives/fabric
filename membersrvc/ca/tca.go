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
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"math"
	"math/big"
	"strconv"
	"database/sql"
	"time"


	protobuf "google/protobuf"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/abac"
	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	
    "google/protobuf"

)

var (
	// TCertEncTCertIndex is the ASN1 object identifier of the TCert index.
	TCertEncTCertIndex = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 7}

	// TCertEncEnrollmentID is the ASN1 object identifier of the enrollment id.
	TCertEncEnrollmentID = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 8}	

	// TCertAttributesHeaders is the ASN1 object identifier of attributes header.
	TCertAttributesHeaders = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 9}

	// Padding for encryption.
	Padding = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
)

// TCA is the transaction certificate authority.
type TCA struct {
	*CA
	eca        *ECA
	hmacKey    []byte
	rootPreKey []byte
	preKeys    map[string][]byte
}

// TCAP serves the public GRPC interface of the TCA.
type TCAP struct {
	tca *TCA
}

// TCAA serves the administrator GRPC interface of the TCA.
type TCAA struct {
	tca *TCA
}

func initializeTCATables(db *sql.DB) error { 
	return initializeCommonTables(db)
}


// NewTCA sets up a new TCA.
func NewTCA(eca *ECA) *TCA {
	tca := &TCA{NewCA("tca", initializeTCATables), eca, nil, nil, nil}

	err := tca.readHmacKey()
	if err != nil {
		Panic.Panicln(err)
	}

	err = tca.readRootPreKey()
	if err != nil {
		Panic.Panicln(err)
	}

	err = tca.initializePreKeyTree()
	if err != nil {
		Panic.Panicln(err)
	}
	return tca
}

// Read the hcmac key from the file system.
func (tca *TCA) readHmacKey() error {
	var cooked string
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
	return err
}

// Read the root pre key from the file system.
func (tca *TCA) readRootPreKey() error {
	var cooked string
	raw, err := ioutil.ReadFile(tca.path + "/root_pk.hmac")
	if err != nil {
		key := make([]byte, 49)
		rand.Reader.Read(key)
		cooked = base64.StdEncoding.EncodeToString(key)

		err = ioutil.WriteFile(tca.path+"/root_pk.hmac", []byte(cooked), 0644)
		if err != nil {
			Panic.Panicln(err)
		}
	} else {
		cooked = string(raw)
	}

	tca.rootPreKey, err = base64.StdEncoding.DecodeString(cooked)
	return err
}

func (tca *TCA) calculatePreKey(variant []byte, preKey []byte) ([]byte, error) {
	mac := hmac.New(primitives.GetDefaultHash(), preKey)
	_, err := mac.Write(variant)
	if err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func (tca *TCA) initializePreKeyNonRootGroup(group *AffiliationGroup) error {
	if group.parent.preKey == nil {
		//Initialize parent if it is not initialized yet.
		tca.initializePreKeyGroup(group.parent)
	}
	var err error
	group.preKey, err = tca.calculatePreKey([]byte(group.name), group.parent.preKey)
	return err
}

func (tca *TCA) initializePreKeyGroup(group *AffiliationGroup) error {
	if group.parentId == 0 {
		//This group is root
		group.preKey = tca.rootPreKey
		return nil
	} else {
		return tca.initializePreKeyNonRootGroup(group)
	}
}

func (tca *TCA) initializePreKeyTree() error {
	Trace.Println("Initializing PreKeys.")
	groups, err := tca.eca.readAffiliationGroups()
	if err != nil {
		return err
	}
	tca.preKeys = make(map[string][]byte)
	for _, group := range groups {
		if group.preKey == nil {
			err = tca.initializePreKeyGroup(group)
			if err != nil {
				return err
			}
		}
		Trace.Println("Initializing PK group ", group.name)
		tca.preKeys[group.name] = group.preKey
	}

	return nil
}

func (tca *TCA) getPreKFrom(enrollmentCertificate *x509.Certificate) ([]byte, error) {
	_, _, affiliation, err := tca.eca.parseEnrollId(enrollmentCertificate.Subject.CommonName)
	if err != nil {
		return nil, err
	}
	preK := tca.preKeys[affiliation]
	if preK == nil {
		return nil, errors.New("Could not be found a pre-k to the affiliation group " + affiliation + ".")
	}
	return preK, nil
}

// Start starts the TCA.
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
func (tcap *TCAP) ReadCACertificate(ctx context.Context, in *pb.Empty) (*pb.Cert, error) {
	Trace.Println("grpc TCAP:ReadCACertificate")

	return &pb.Cert{tcap.tca.raw}, nil
}


func (tcap *TCAP) selectValidAttributes(cert_raw []byte) ([]*pb.TCertAttribute, error) { 
	cert, err := x509.ParseCertificate(cert_raw)
	if err != nil { 
		return nil, err
	}
	
	ans := make([]*pb.TCertAttribute, 0)
	
	if cert.Extensions == nil { 
		return ans, nil
	}
	currentTime := time.Now()
	for _, extension := range(cert.Extensions) { 
		acaAtt := &pb.ACAAttribute{"", nil ,&google_protobuf.Timestamp{0,0},&google_protobuf.Timestamp{0,0}}
		
		if IsAttributeOID(extension.Id)  {
				if err := proto.Unmarshal(extension.Value, acaAtt); err != nil { 
					continue
				}	
					
				if acaAtt.AttributeName == "" { 
					continue
				}
				var from, to time.Time
				if acaAtt.ValidFrom != nil { 
					from = time.Unix(acaAtt.ValidFrom.Seconds,int64(acaAtt.ValidFrom.Nanos)) 
				}
				if acaAtt.ValidTo != nil { 
					to = time.Unix(acaAtt.ValidTo.Seconds,int64(acaAtt.ValidTo.Nanos)) 	
				}
			
				//Check if the attribute still being valid.
				if (from.Before(currentTime) || from.Equal(currentTime)) && (to.IsZero() || to.After(currentTime)) { 
					ans = append(ans, &pb.TCertAttribute{acaAtt.AttributeName, string(acaAtt.AttributeValue)})
				}
			}
	}
	return ans, nil
}

func (tcap *TCAP) requestAttributes(id string, ecert []byte, attributes []*pb.TCertAttribute ) ([]*pb.TCertAttribute, error) { 
	//TODO we are creation a new client connection per each ecer request. We should be implement a connections pool.
	sock, acaP, err := GetACAClient()
	if err != nil { 
		return nil, err
	}
	defer sock.Close() 
	attributesHash := make([]*pb.TCertAttributeHash, 0)
	
	for _, att := range(attributes) {
		attributeHash := pb.TCertAttributeHash{att.AttributeName, primitives.Hash([]byte(att.AttributeValue))}
		attributesHash = append(attributesHash, &attributeHash)
	}
	
	req := &pb.ACAAttrReq{
		Ts: &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id: &pb.Identity{id},
		ECert: &pb.Cert{ecert},
		Attributes: attributesHash,
		Signature:  nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	
	var r,s *big.Int
	
	r,s, err = primitives.ECDSASignDirect(tcap.tca.priv, rawReq) 
	
	if err != nil {
		return  nil, err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp , err := acaP.RequestAttributes(context.Background(),  req)
	if err != nil { 
		return nil, err
	}

	if resp.Status == pb.ACAAttrResp_FAILURE {
		return nil,  errors.New("Error fetching attributes.")
	} 
		
	return tcap.selectValidAttributes(resp.Cert.Cert)
	
}

// CreateCertificateSet requests the creation of a new transaction certificate set by the TCA.
func (tcap *TCAP) CreateCertificateSet(ctx context.Context, in *pb.TCertCreateSetReq) (*pb.TCertCreateSetResp, error) {
	Trace.Println("grpc TCAP:CreateCertificateSet")

	id := in.Id.Id
	raw, err := tcap.tca.eca.readCertificate(id, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	
	var attributes = []*pb.TCertAttribute{}
	if in.Attributes != nil {
		attributes, err = tcap.requestAttributes(id,raw, in.Attributes)
		if err != nil { 
			return nil, err
		}
	}	
	
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	
	pub := cert.PublicKey.(*ecdsa.PublicKey)

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(in.Sig.R)
	s.UnmarshalText(in.Sig.S)

	//sig := in.Sig
	in.Sig = nil

	hash := primitives.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(pub, hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	// Generate nonce for TCertIndex
	nonce := make([]byte, 16) // 8 bytes rand, 8 bytes timestamp
	rand.Reader.Read(nonce[:8])

	mac := hmac.New(primitives.GetDefaultHash(), tcap.tca.hmacKey)
	raw, _ = x509.MarshalPKIXPublicKey(pub)
	mac.Write(raw)
	kdfKey := mac.Sum(nil)

	num := int(in.Num)
	if num == 0 {
		num = 1
	}

	// the batch of TCerts
	var set []*pb.TCert

	for i := 0; i < num; i++ {
		tcertid := util.GenerateIntUUID()

		// Compute TCertIndex
		tidx := []byte(strconv.Itoa(2*i + 1))
		tidx = append(tidx[:], nonce[:]...)
		tidx = append(tidx[:], Padding...)

		mac := hmac.New(primitives.GetDefaultHash(), kdfKey)
		mac.Write([]byte{1})
		extKey := mac.Sum(nil)[:32]

		mac = hmac.New(primitives.GetDefaultHash(), kdfKey)
		mac.Write([]byte{2})
		mac = hmac.New(primitives.GetDefaultHash(), mac.Sum(nil))
		mac.Write(tidx)

		one := new(big.Int).SetInt64(1)
		k := new(big.Int).SetBytes(mac.Sum(nil))
		k.Mod(k, new(big.Int).Sub(pub.Curve.Params().N, one))
		k.Add(k, one)

		tmpX, tmpY := pub.ScalarBaseMult(k.Bytes())
		txX, txY := pub.Curve.Add(pub.X, pub.Y, tmpX, tmpY)
		txPub := ecdsa.PublicKey{Curve: pub.Curve, X: txX, Y: txY}

		// Compute encrypted TCertIndex
		encryptedTidx, err := CBCEncrypt(extKey, tidx)
		if err != nil {
			return nil, err
		}
		
		extensions, pre_K0, err := tcap.generateExtensions(tcertid, encryptedTidx, cert, attributes)

		if err != nil {
			return nil, err
		}

		spec := NewDefaultPeriodCertificateSpec(id, tcertid, &txPub, x509.KeyUsageDigitalSignature, extensions...)
		if raw, err = tcap.tca.createCertificateFromSpec(spec, in.Ts.Seconds, kdfKey); err != nil {
			Error.Println(err)
			return nil, err
		}

		set = append(set, &pb.TCert{raw, pre_K0})
	}

	return &pb.TCertCreateSetResp{&pb.CertSet{in.Ts, in.Id, kdfKey, set}}, nil
}

// Generate encrypted extensions to be included into the TCert (TCertIndex, EnrollmentID and attributes).
func (tcap *TCAP) generateExtensions(tcertid *big.Int, tidx []byte, enrollmentCert *x509.Certificate, attributes []*pb.TCertAttribute) ([]pkix.Extension, []byte, error){
	// For each TCert we need to store and retrieve to the user the list of Ks used to encrypt the EnrollmentID and the attributes.
	extensions := make([]pkix.Extension, len(attributes))

	// Compute preK_1 to encrypt attributes and enrollment ID
	preK_1, err := tcap.tca.getPreKFrom(enrollmentCert)
	if err != nil {
		return nil, nil, err
	}

	mac := hmac.New(primitives.GetDefaultHash(), preK_1)
	mac.Write(tcertid.Bytes())
	preK_0 := mac.Sum(nil)

	// Compute encrypted EnrollmentID
	mac = hmac.New(primitives.GetDefaultHash(), preK_0)
	mac.Write([]byte("enrollmentID"))
	enrollmentIdKey := mac.Sum(nil)[:32]

	enrollmentID := []byte(enrollmentCert.Subject.CommonName)
	enrollmentID = append(enrollmentID, Padding...)

	encEnrollmentID, err := CBCEncrypt(enrollmentIdKey, enrollmentID)
	if err != nil {
		return nil, nil, err
	}

	// save k used to encrypt EnrollmentID
	//ks["enrollmentId"] = enrollmentIdKey
	
	attributeIdentifierIndex := 9
	count := 0
	attributesHeader := make(map[string]int)
	// Encrypt and append attributes to the extensions slice
	for _, a := range attributes {
		count++

		value := []byte(a.AttributeValue)

		//Save the position of the attribute extension on the header.
		attributesHeader[a.AttributeName] = count

		if viper.GetBool("tca.attribute-encryption.enabled") {
			value, err = abac.EncryptAttributeValuePK0(preK_0, a.AttributeName, value)
			if err != nil {
				return nil, nil, err
			}
		}

		// Generate an ObjectIdentifier for the extension holding the attribute
		TCertEncAttributes := asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, attributeIdentifierIndex + count}

		// Add the attribute extension to the extensions array
		extensions[count-1] = pkix.Extension{Id: TCertEncAttributes, Critical: false, Value: value}
	}

	// Append the TCertIndex to the extensions
	extensions = append(extensions, pkix.Extension{Id: TCertEncTCertIndex, Critical: true, Value: tidx})

	// Append the encrypted EnrollmentID to the extensions
	extensions = append(extensions, pkix.Extension{Id: TCertEncEnrollmentID, Critical: false, Value: encEnrollmentID})

	// Append the attributes header if there was attributes to include in the TCert
	if len(attributes) > 0 {
		headerValue := abac.BuildAttributesHeader(attributesHeader)
		if viper.GetBool("tca.attribute-encryption.enabled") {
			headerValue, err = abac.EncryptAttributeValuePK0(preK_0, abac.HeaderAttributeName, headerValue)
			if err != nil {
				return nil, nil, err
			}
		}
		extensions = append(extensions, pkix.Extension{Id: TCertAttributesHeaders, Critical: false, Value: headerValue})
	}
	
	return extensions, preK_0, nil
}

// ReadCertificate reads a transaction certificate from the TCA.
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

	hash := primitives.NewHash()
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

	hash := primitives.NewHash()
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

	var certs []*pb.TCert
	var kdfKey []byte
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw, &kdfKey); err != nil {
			return nil, err
		}

		// TODO: TCert must include attribute keys, we need to save them in the db when generating the batch of TCerts
		certs = append(certs, &pb.TCert{raw, make([]byte, 48)})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &pb.CertSet{in.Ts, in.Id, kdfKey, certs}, nil
}

// RevokeCertificate revokes a certificate from the TCA.  Not yet implemented.
func (tcap *TCAP) RevokeCertificate(context.Context, *pb.TCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificateSet revokes a certificate set from the TCA.  Not yet implemented.
func (tcap *TCAP) RevokeCertificateSet(context.Context, *pb.TCertRevokeSetReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAP:RevokeCertificateSet")

	return nil, errors.New("not yet implemented")
}

// ReadCertificateSets returns all certificates matching the filter criteria of the request.
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

	hash := primitives.NewHash()
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

		var certs []*pb.TCert
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

			// TODO: TCert must include attribute keys, we need to save them in the db when generating the batch of TCerts
			certs = append(certs, &pb.TCert{cert, make([]byte, 48)})
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
func (tcaa *TCAA) RevokeCertificate(context.Context, *pb.TCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificateSet revokes a certificate set from the TCA.  Not yet implemented.
func (tcaa *TCAA) RevokeCertificateSet(context.Context, *pb.TCertRevokeSetReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:RevokeCertificateSet")

	return nil, errors.New("not yet implemented")
}

// PublishCRL requests the creation of a certificate revocation list from the TCA.  Not yet implemented.
func (tcaa *TCAA) PublishCRL(context.Context, *pb.TCertCRLReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:CreateCRL")

	return nil, errors.New("not yet implemented")
}

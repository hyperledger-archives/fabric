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
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/attributes"
	"github.com/hyperledger/fabric/core/crypto/primitives"
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

	// RootPreKeySize for attribute encryption keys derivation
	RootPreKeySize = 48
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

// TCertSet contains relevant information of a set of tcerts
type TCertSet struct {
	Ts           int64
	EnrollmentID string
	Nonce        []byte
	Key          []byte
}

func initializeTCATables(db *sql.DB) error {
	var err error

	err = initializeCommonTables(db)
	if err != nil {
		return err
	}

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS TCertificateSets (row INTEGER PRIMARY KEY, enrollmentID VARCHAR(64), timestamp INTEGER, nonce BLOB, kdfkey BLOB)"); err != nil {
		return err
	}

	return err
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
		key := make([]byte, RootPreKeySize)
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
	if group.parentID == 0 {
		//This group is root
		group.preKey = tca.rootPreKey
		return nil
	}
	return tca.initializePreKeyNonRootGroup(group)
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
	_, _, affiliation, err := tca.eca.parseEnrollID(enrollmentCertificate.Subject.CommonName)
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

	Info.Println("TCA started.")
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

	return &pb.Cert{Cert: tcap.tca.raw}, nil
}

func (tcap *TCAP) selectValidAttributes(certRaw []byte) ([]*pb.ACAAttribute, error) {
	cert, err := x509.ParseCertificate(certRaw)
	if err != nil {
		return nil, err
	}

	var ans []*pb.ACAAttribute

	if cert.Extensions == nil {
		return ans, nil
	}
	currentTime := time.Now()
	for _, extension := range cert.Extensions {
		acaAtt := &pb.ACAAttribute{AttributeName: "", AttributeValue: nil, ValidFrom: &google_protobuf.Timestamp{Seconds: 0, Nanos: 0}, ValidTo: &google_protobuf.Timestamp{Seconds: 0, Nanos: 0}}

		if IsAttributeOID(extension.Id) {
			if err := proto.Unmarshal(extension.Value, acaAtt); err != nil {
				continue
			}

			if acaAtt.AttributeName == "" {
				continue
			}
			var from, to time.Time
			if acaAtt.ValidFrom != nil {
				from = time.Unix(acaAtt.ValidFrom.Seconds, int64(acaAtt.ValidFrom.Nanos))
			}
			if acaAtt.ValidTo != nil {
				to = time.Unix(acaAtt.ValidTo.Seconds, int64(acaAtt.ValidTo.Nanos))
			}

			//Check if the attribute still being valid.
			if (from.Before(currentTime) || from.Equal(currentTime)) && (to.IsZero() || to.After(currentTime)) {
				ans = append(ans, acaAtt)
			}
		}
	}
	return ans, nil
}

func (tcap *TCAP) requestAttributes(id string, ecert []byte, attrs []*pb.TCertAttribute) ([]*pb.ACAAttribute, error) {
	//TODO we are creation a new client connection per each ecer request. We should be implement a connections pool.
	sock, acaP, err := GetACAClient()
	if err != nil {
		return nil, err
	}
	defer sock.Close()
	var attrNames []*pb.TCertAttribute

	for _, att := range attrs {
		attrName := pb.TCertAttribute{AttributeName: att.AttributeName}
		attrNames = append(attrNames, &attrName)
	}

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: id},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attrNames,
		Signature:  nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	var r, s *big.Int

	r, s, err = primitives.ECDSASignDirect(tcap.tca.priv, rawReq)

	if err != nil {
		return nil, err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := acaP.RequestAttributes(context.Background(), req)
	if err != nil {
		return nil, err
	}

	if resp.Status >= pb.ACAAttrResp_FAILURE_MINVAL && resp.Status <= pb.ACAAttrResp_FAILURE_MAXVAL {
		return nil, errors.New(fmt.Sprint("Error fetching attributes = ", resp.Status))
	}

	return tcap.selectValidAttributes(resp.Cert.Cert)
}

// CreateCertificateSet requests the creation of a new transaction certificate set by the TCA.
func (tcap *TCAP) CreateCertificateSet(ctx context.Context, in *pb.TCertCreateSetReq) (*pb.TCertCreateSetResp, error) {
	Trace.Println("grpc TCAP:CreateCertificateSet")

	id := in.Id.Id
	raw, err := tcap.tca.eca.readCertificateByKeyUsage(id, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}

	return tcap.createCertificateSet(ctx, raw, in)
}

func (tcap *TCAP) createCertificateSet(ctx context.Context, raw []byte, in *pb.TCertCreateSetReq) (*pb.TCertCreateSetResp, error) {
	var attrs = []*pb.ACAAttribute{}
	var err error
	var id = in.Id.Id
	var timestamp = in.Ts.Seconds

	if in.Attributes != nil && viper.GetBool("aca.enabled") {
		attrs, err = tcap.requestAttributes(id, raw, in.Attributes)
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

		extensions, preK0, err := tcap.generateExtensions(tcertid, encryptedTidx, cert, attrs)

		if err != nil {
			return nil, err
		}

		spec := NewDefaultPeriodCertificateSpec(id, tcertid, &txPub, x509.KeyUsageDigitalSignature, extensions...)
		if raw, err = tcap.tca.createCertificateFromSpec(spec, timestamp, kdfKey, false); err != nil {
			Error.Println(err)
			return nil, err
		}

		set = append(set, &pb.TCert{Cert: raw, Prek0: preK0})
	}

	tcap.tca.persistCertificateSet(id, timestamp, nonce, kdfKey)

	return &pb.TCertCreateSetResp{Certs: &pb.CertSet{Ts: in.Ts, Id: in.Id, Key: kdfKey, Certs: set}}, nil
}

func (tca *TCA) getCertificateSets(enrollmentID string) ([]*TCertSet, error) {
	var sets = []*TCertSet{}
	var err error

	var rows *sql.Rows
	rows, err = tca.retrieveCertificateSets(enrollmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enrollID string
	var timestamp int64
	var nonce []byte
	var kdfKey []byte

	for rows.Next() {
		if err = rows.Scan(&enrollID, &timestamp, &nonce, &kdfKey); err != nil {
			return nil, err
		}
		sets = append(sets, &TCertSet{Ts: timestamp, EnrollmentID: enrollID, Key: kdfKey})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return sets, nil
}

func (tca *TCA) persistCertificateSet(enrollmentID string, timestamp int64, nonce []byte, kdfKey []byte) error {
	var err error

	if _, err = tca.db.Exec("INSERT INTO TCertificateSets (enrollmentID, timestamp, nonce, kdfkey) VALUES (?, ?, ?, ?)", enrollmentID, timestamp, nonce, kdfKey); err != nil {
		Error.Println(err)
	}
	return err
}

func (tca *TCA) retrieveCertificateSets(enrollmentID string) (*sql.Rows, error) {
	return tca.db.Query("SELECT enrollmentID, timestamp, nonce, kdfkey FROM TCertificateSets WHERE enrollmentID=?", enrollmentID)
}

// Generate encrypted extensions to be included into the TCert (TCertIndex, EnrollmentID and attributes).
func (tcap *TCAP) generateExtensions(tcertid *big.Int, tidx []byte, enrollmentCert *x509.Certificate, attrs []*pb.ACAAttribute) ([]pkix.Extension, []byte, error) {
	// For each TCert we need to store and retrieve to the user the list of Ks used to encrypt the EnrollmentID and the attributes.
	extensions := make([]pkix.Extension, len(attrs))

	// Compute preK_1 to encrypt attributes and enrollment ID
	preK1, err := tcap.tca.getPreKFrom(enrollmentCert)
	if err != nil {
		return nil, nil, err
	}

	mac := hmac.New(primitives.GetDefaultHash(), preK1)
	mac.Write(tcertid.Bytes())
	preK0 := mac.Sum(nil)

	// Compute encrypted EnrollmentID
	mac = hmac.New(primitives.GetDefaultHash(), preK0)
	mac.Write([]byte("enrollmentID"))
	enrollmentIDKey := mac.Sum(nil)[:32]

	enrollmentID := []byte(enrollmentCert.Subject.CommonName)
	enrollmentID = append(enrollmentID, Padding...)

	encEnrollmentID, err := CBCEncrypt(enrollmentIDKey, enrollmentID)
	if err != nil {
		return nil, nil, err
	}

	attributeIdentifierIndex := 9
	count := 0
	attrsHeader := make(map[string]int)
	// Encrypt and append attrs to the extensions slice
	for _, a := range attrs {
		count++

		value := []byte(a.AttributeValue)

		//Save the position of the attribute extension on the header.
		attrsHeader[a.AttributeName] = count

		if isEnabledAttributesEncryption() {
			value, err = attributes.EncryptAttributeValuePK0(preK0, a.AttributeName, value)
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
	if len(attrs) > 0 {
		headerValue, err := attributes.BuildAttributesHeader(attrsHeader)
		if err != nil {
			return nil, nil, err
		}
		if isEnabledAttributesEncryption() {
			headerValue, err = attributes.EncryptAttributeValuePK0(preK0, attributes.HeaderAttributeName, headerValue)
			if err != nil {
				return nil, nil, err
			}
		}
		extensions = append(extensions, pkix.Extension{Id: TCertAttributesHeaders, Critical: false, Value: headerValue})
	}

	return extensions, preK0, nil
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

func isEnabledAttributesEncryption() bool {
	//TODO this code is commented because attributes encryption is not yet implemented.
	//return viper.GetBool("tca.attribute-encryption.enabled")
	return false
}

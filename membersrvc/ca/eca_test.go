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
	"crypto/rand"
	"crypto/x509"
	"os"
	"testing"
	"time"

	protobuf "google/protobuf"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/primitives/ecies"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"golang.org/x/net/context"
)

type User struct {
	enrollID        string
	enrollPwd       []byte
	enrollPrivKey   *ecdsa.PrivateKey
	role            int
	affiliation     string
	affiliationRole string
}

var (
	ecaFiles    = [6]string{"eca.cert", "eca.db", "eca.priv", "eca.pub", "obc.aes", "obc.ecies"}
	testUser    = User{enrollID: "testUser", role: 1, affiliation: "institution_a", affiliationRole: "00001"}
	testAuditor = User{enrollID: "testAuditor", enrollPwd: []byte("NPKYL39uKbkj")}
)

//helper function for multiple tests
func enrollUser(user *User) error {

	ecap := &ECAP{eca}

	//Phase 1 of the protocol
	//generate crypto material
	signPriv, err := primitives.NewECDSAKey()
	user.enrollPrivKey = signPriv
	if err != nil {
		return err
	}
	signPub, err := x509.MarshalPKIXPublicKey(&signPriv.PublicKey)
	if err != nil {
		return err
	}

	encPriv, err := primitives.NewECDSAKey()
	if err != nil {
		return err
	}
	encPub, err := x509.MarshalPKIXPublicKey(&encPriv.PublicKey)
	if err != nil {
		return err
	}

	req := &pb.ECertCreateReq{
		Ts:   &protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:   &pb.Identity{Id: user.enrollID},
		Tok:  &pb.Token{Tok: user.enrollPwd},
		Sign: &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: signPub},
		Enc:  &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: encPub},
		Sig:  nil}

	resp, err := ecap.CreateCertificatePair(context.Background(), req)
	if err != nil {
		return err
	}

	//Phase 2 of the protocol
	spi := ecies.NewSPI()
	eciesKey, err := spi.NewPrivateKey(nil, encPriv)
	if err != nil {
		return err
	}

	ecies, err := spi.NewAsymmetricCipherFromPublicKey(eciesKey)
	if err != nil {
		return err
	}

	out, err := ecies.Process(resp.Tok.Tok)
	if err != nil {
		return err
	}

	req.Tok.Tok = out
	req.Sig = nil

	hash := primitives.NewHash()
	raw, _ := proto.Marshal(req)
	hash.Write(raw)

	r, s, err := ecdsa.Sign(rand.Reader, signPriv, hash.Sum(nil))
	if err != nil {
		return err
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err = ecap.CreateCertificatePair(context.Background(), req)
	if err != nil {
		return err
	}

	//verify we got vaild crypto material back
	x509SignCert, err := primitives.DERToX509Certificate(resp.Certs.Sign)
	if err != nil {
		return err
	}

	_, err = primitives.GetCriticalExtension(x509SignCert, ECertSubjectRole)
	if err != nil {
		return err
	}

	x509EncCert, err := primitives.DERToX509Certificate(resp.Certs.Enc)
	if err != nil {
		return err
	}

	_, err = primitives.GetCriticalExtension(x509EncCert, ECertSubjectRole)
	if err != nil {
		return err
	}

	return nil
}

//check that the ECA was created / initialized
func TestNewECA(t *testing.T) {

	//initialization was handled in TestMain
	//check to see if ECA exists
	if eca != nil {

		missing := 0
		//check to see that the expected files were created
		for _, file := range ecaFiles {
			if _, err := os.Stat(eca.CA.path + "/" + file); err != nil {
				missing++
				t.Logf("failed to find file [%s]", file)
			}
		}

		if missing > 0 {
			t.FailNow()
		}

	} else {
		t.Error("Failed to create ECA")
	}
}

//register testUser
func TestRegisterUser(t *testing.T) {

	ecaa := &ECAA{eca}

	//create req
	req := &pb.RegisterUserReq{
		Id:          &pb.Identity{Id: testUser.enrollID},
		Role:        pb.Role(testUser.role),
		Account:     testUser.affiliation,
		Affiliation: testUser.affiliationRole}

	token, err := ecaa.RegisterUser(context.Background(), req)
	if err != nil {
		t.Errorf("Failed to register user: [%s]", err.Error())
	}

	if token != nil {
		testUser.enrollPwd = token.Tok
	} else {
		t.Error("Failed to obtain token")
	}

}

//register testUser again - should get error
func TestRegisterDuplicateUser(t *testing.T) {

	ecaa := &ECAA{eca}

	//create req
	req := &pb.RegisterUserReq{
		Id:          &pb.Identity{Id: testUser.enrollID},
		Role:        pb.Role(testUser.role),
		Account:     testUser.affiliation,
		Affiliation: testUser.affiliationRole}

	_, err := ecaa.RegisterUser(context.Background(), req)
	if err.Error() != "user is already registered" {
		t.Errorf("Expected error was not returned when registering user twice: [%s]", err.Error())
	}
}

func TestReadCACertificate(t *testing.T) {

	ecap := &ECAP{eca}
	_, err := ecap.ReadCACertificate(context.Background(), &pb.Empty{})

	if err != nil {
		t.Errorf("Failed to read ECA CA certificate: [%s]", err.Error())
	}

}

func TestCreateCertificatePair(t *testing.T) {

	ecap := &ECAP{eca}

	//Phase 1 of the protocol
	//generate crypto material
	signPriv, err := primitives.NewECDSAKey()
	testUser.enrollPrivKey = signPriv
	if err != nil {
		t.Errorf("Failed generating ECDSA key [%s].", err.Error())
	}
	signPub, err := x509.MarshalPKIXPublicKey(&signPriv.PublicKey)
	if err != nil {
		t.Errorf("Failed mashalling ECDSA key [%s].", err.Error())
	}

	encPriv, err := primitives.NewECDSAKey()
	if err != nil {
		t.Errorf("Failed generating Encryption key [%s].", err.Error())
	}
	encPub, err := x509.MarshalPKIXPublicKey(&encPriv.PublicKey)
	if err != nil {
		t.Errorf("Failed marshalling Encryption key [%s].", err.Error())
	}

	req := &pb.ECertCreateReq{
		Ts:   &protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:   &pb.Identity{Id: testUser.enrollID},
		Tok:  &pb.Token{Tok: testUser.enrollPwd},
		Sign: &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: signPub},
		Enc:  &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: encPub},
		Sig:  nil}

	resp, err := ecap.CreateCertificatePair(context.Background(), req)
	if err != nil {
		t.Errorf("Failed to enroll user: [%s]", err.Error())
	}

	//Phase 2 of the protocol
	spi := ecies.NewSPI()
	eciesKey, err := spi.NewPrivateKey(nil, encPriv)
	if err != nil {
		t.Errorf("Failed parsing decrypting key [%s].", err.Error())
	}

	ecies, err := spi.NewAsymmetricCipherFromPublicKey(eciesKey)
	if err != nil {
		t.Errorf("Failed creating asymmetrinc cipher [%s].", err.Error())
	}

	out, err := ecies.Process(resp.Tok.Tok)
	if err != nil {
		t.Errorf("Failed decrypting toke [%s].", err.Error())
	}

	req.Tok.Tok = out
	req.Sig = nil

	hash := primitives.NewHash()
	raw, _ := proto.Marshal(req)
	hash.Write(raw)

	r, s, err := ecdsa.Sign(rand.Reader, signPriv, hash.Sum(nil))
	if err != nil {
		t.Errorf("Failed signing [%s].", err.Error())
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err = ecap.CreateCertificatePair(context.Background(), req)
	if err != nil {
		t.Errorf("Failed to enroll user: [%s]", err.Error())
	}

	//verify we got vaild crypto material back
	x509SignCert, err := primitives.DERToX509Certificate(resp.Certs.Sign)
	if err != nil {
		t.Errorf("Failed parsing signing enrollment certificate for signing: [%s]", err.Error())
	}

	_, err = primitives.GetCriticalExtension(x509SignCert, ECertSubjectRole)
	if err != nil {
		t.Errorf("Failed parsing ECertSubjectRole in enrollment certificate for signing: [%s]", err.Error())
	}

	x509EncCert, err := primitives.DERToX509Certificate(resp.Certs.Enc)
	if err != nil {
		t.Errorf("Failed parsing signing enrollment certificate for encrypting: [%s]", err.Error())
	}

	_, err = primitives.GetCriticalExtension(x509EncCert, ECertSubjectRole)
	if err != nil {
		t.Errorf("Failed parsing ECertSubjectRole in enrollment certificate for encrypting: [%s]", err.Error())
	}

}

func TestReadCertificatePair(t *testing.T) {

	ecap := &ECAP{eca}

	req := &pb.ECertReadReq{Id: &pb.Identity{Id: testUser.enrollID}}

	_, err := ecap.ReadCertificatePair(context.Background(), req)

	if err != nil {
		t.Errorf("Failed to read certificate pair: [%s]", err.Error())
	}

}

func TestReadCertificatePairBadIdentity(t *testing.T) {
	t.SkipNow() //need to fix error
	ecap := &ECAP{eca}

	req := &pb.ECertReadReq{Id: &pb.Identity{Id: "badUser"}}

	_, err := ecap.ReadCertificatePair(context.Background(), req)

	if err != nil {
		t.Errorf("Failed to read certificate pair: [%s]", err.Error())
	}

}

func TestReadUserSet(t *testing.T) {

	//enroll Auditor
	err := enrollUser(&testAuditor)

	if err != nil {
		t.Errorf("Failed to read user set: [%s]", err.Error())
	}

	ecaa := &ECAA{eca}

	req := &pb.ReadUserSetReq{
		Req:  &pb.Identity{Id: testAuditor.enrollID},
		Role: 1,
		Sig:  nil}

	//sign the req
	hash := primitives.NewHash()
	raw, _ := proto.Marshal(req)
	hash.Write(raw)

	r, s, err := ecdsa.Sign(rand.Reader, testAuditor.enrollPrivKey, hash.Sum(nil))
	if err != nil {
		t.Errorf("Failed signing [%s].", err.Error())
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := ecaa.ReadUserSet(context.Background(), req)

	if err != nil {
		t.Errorf("Failed to read user set: [%s]", err.Error())
	}

	t.Logf("number of users: [%s]", len(resp.Users))

}

func TestReadUserSetNonAuditor(t *testing.T) {

	ecaa := &ECAA{eca}

	req := &pb.ReadUserSetReq{
		Req:  &pb.Identity{Id: testUser.enrollID},
		Role: 1,
		Sig:  nil}

	//sign the req
	hash := primitives.NewHash()
	raw, _ := proto.Marshal(req)
	hash.Write(raw)

	r, s, err := ecdsa.Sign(rand.Reader, testUser.enrollPrivKey, hash.Sum(nil))
	if err != nil {
		t.Errorf("Failed signing [%s].", err.Error())
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	_, err = ecaa.ReadUserSet(context.Background(), req)

	if err == nil {
		t.Error("Only auditors should be able to call ReadUserSet")
	}

}

func TestCreateCertificatePairBadIdentity(t *testing.T) {

	ecap := &ECAP{eca}

	req := &pb.ECertCreateReq{
		Ts:   &protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:   &pb.Identity{Id: "badIdentity"},
		Tok:  &pb.Token{Tok: testUser.enrollPwd},
		Sign: &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: []byte{0}},
		Enc:  &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: []byte{0}},
		Sig:  nil}

	_, err := ecap.CreateCertificatePair(context.Background(), req)
	if err.Error() != "Identity or token does not match." {
		t.Error("Expected error was not returned for bad identity")
	}
}

func TestCreateCertificatePairBadToken(t *testing.T) {

	ecap := &ECAP{eca}

	req := &pb.ECertCreateReq{
		Ts:   &protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:   &pb.Identity{Id: testUser.enrollID},
		Tok:  &pb.Token{Tok: []byte("badPassword")},
		Sign: &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: []byte{0}},
		Enc:  &pb.PublicKey{Type: pb.CryptoType_ECDSA, Key: []byte{0}},
		Sig:  nil}

	_, err := ecap.CreateCertificatePair(context.Background(), req)
	if err.Error() != "Identity or token does not match." {
		t.Error("Expected error was not returned for bad password")
	}
}

func TestRevokeCertificatePair(t *testing.T) {

	ecap := &ECAP{eca}

	_, err := ecap.RevokeCertificatePair(context.Background(), &pb.ECertRevokeReq{})
	if err.Error() != "ECAP:RevokeCertificate method not (yet) implemented" {
		t.Errorf("Expected error was not returned: [%s]", err.Error())
	}
}

func TestRevokeCertificate(t *testing.T) {

	ecaa := &ECAA{eca}

	_, err := ecaa.RevokeCertificate(context.Background(), &pb.ECertRevokeReq{})
	if err.Error() != "ECAA:RevokeCertificate method not (yet) implemented" {
		t.Errorf("Expected error was not returned: [%s]", err.Error())
	}
}

func TestPublishCRL(t *testing.T) {
	ecaa := &ECAA{eca}

	_, err := ecaa.PublishCRL(context.Background(), &pb.ECertCRLReq{})
	if err.Error() != "ECAA:PublishCRL method not (yet) implemented" {
		t.Error("Expected error was not returned: [%s]", err.Error())
	}
}

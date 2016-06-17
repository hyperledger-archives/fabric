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
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/subtle"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"google/protobuf"
	"io/ioutil"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/crypto/primitives/ecies"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	// ECertSubjectRole is the ASN1 object identifier of the subject's role.
	//
	ECertSubjectRole = asn1.ObjectIdentifier{2, 1, 3, 4, 5, 6, 7}
)

// ECA is the enrollment certificate authority.
//
type ECA struct {
	*CA
	obcKey          []byte
	obcPriv, obcPub []byte
}

// ECAP serves the public GRPC interface of the ECA.
//
type ECAP struct {
	eca *ECA
}

// ECAA serves the administrator GRPC interface of the ECA.
//
type ECAA struct {
	eca *ECA
}

func initializeECATables(db *sql.DB) error {
	return initializeCommonTables(db)
}

// NewECA sets up a new ECA.
//
func NewECA() *ECA {
	eca := &ECA{NewCA("eca", initializeECATables), nil, nil, nil}

	{
		// read or create global symmetric encryption key
		var cooked string

		raw, err := ioutil.ReadFile(eca.path + "/obc.aes")
		if err != nil {
			rand := rand.Reader
			key := make([]byte, 32) // AES-256
			rand.Read(key)
			cooked = base64.StdEncoding.EncodeToString(key)

			err = ioutil.WriteFile(eca.path+"/obc.aes", []byte(cooked), 0644)
			if err != nil {
				Panic.Panicln(err)
			}
		} else {
			cooked = string(raw)
		}

		eca.obcKey, err = base64.StdEncoding.DecodeString(cooked)
		if err != nil {
			Panic.Panicln(err)
		}
	}

	{
		// read or create global ECDSA key pair for ECIES
		var priv *ecdsa.PrivateKey
		cooked, err := ioutil.ReadFile(eca.path + "/obc.ecies")
		if err == nil {
			block, _ := pem.Decode(cooked)
			priv, err = x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				Panic.Panicln(err)
			}
		} else {
			priv, err = ecdsa.GenerateKey(primitives.GetDefaultCurve(), rand.Reader)
			if err != nil {
				Panic.Panicln(err)
			}

			raw, _ := x509.MarshalECPrivateKey(priv)
			cooked = pem.EncodeToMemory(
				&pem.Block{
					Type:  "ECDSA PRIVATE KEY",
					Bytes: raw,
				})
			err := ioutil.WriteFile(eca.path+"/obc.ecies", cooked, 0644)
			if err != nil {
				Panic.Panicln(err)
			}
		}

		eca.obcPriv = cooked
		raw, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		eca.obcPub = pem.EncodeToMemory(
			&pem.Block{
				Type:  "ECDSA PUBLIC KEY",
				Bytes: raw,
			})
	}

	eca.populateAffiliationGroupsTable()
	eca.populateUsersTable()
	return eca
}

// populateUsersTable populates the users table.
//
func (eca *ECA) populateUsersTable() {
	// populate user table
	users := viper.GetStringMapString("eca.users")
	for id, flds := range users {
		vals := strings.Fields(flds)
		role, err := strconv.Atoi(vals[0])
		if err != nil {
			Panic.Panicln(err)
		}
		var affiliation, affiliationRole, memberMetadata, registrar string
		if len(vals) >= 4 {
			affiliation = vals[2]
			affiliationRole = vals[3]
			if len(vals) >= 5 {
				memberMetadata = vals[4]
				if len(vals) >= 6 {
					registrar = vals[5]
				}
			}
		}
		eca.registerUser(id, affiliation, affiliationRole, pb.Role(role), registrar, memberMetadata, vals[1])
	}
}

// populateAffiliationGroup populates the affiliation groups table.
//
func (eca *ECA) populateAffiliationGroup(name, parent, key string, level int) {
	eca.registerAffiliationGroup(name, parent)
	newKey := key + "." + name

	if level == 0 {
		affiliationGroups := viper.GetStringSlice(newKey)
		for ci := range affiliationGroups {
			eca.registerAffiliationGroup(affiliationGroups[ci], name)
		}
	} else {
		affiliationGroups := viper.GetStringMapString(newKey)
		for childName := range affiliationGroups {
			eca.populateAffiliationGroup(childName, name, newKey, level-1)
		}
	}
}

// populateAffiliationGroupsTable populates affiliation groups table.
//
func (eca *ECA) populateAffiliationGroupsTable() {
	key := "eca.affiliations"
	affiliationGroups := viper.GetStringMapString(key)
	for name := range affiliationGroups {
		eca.populateAffiliationGroup(name, "", key, 1)
	}
}

// Start starts the ECA.
//
func (eca *ECA) Start(srv *grpc.Server) {
	eca.startECAP(srv)
	eca.startECAA(srv)

	Info.Println("ECA started.")
}

func (eca *ECA) startECAP(srv *grpc.Server) {
	pb.RegisterECAPServer(srv, &ECAP{eca})
}

func (eca *ECA) startECAA(srv *grpc.Server) {
	pb.RegisterECAAServer(srv, &ECAA{eca})
}

// ReadCACertificate reads the certificate of the ECA.
//
func (ecap *ECAP) ReadCACertificate(ctx context.Context, in *pb.Empty) (*pb.Cert, error) {
	Trace.Println("gRPC ECAP:ReadCACertificate")

	return &pb.Cert{Cert: ecap.eca.raw}, nil
}

func (ecap *ECAP) fetchAttributes(cert *pb.Cert) error {
	//TODO we are creating a new client connection per each ecert request. We should implement a connections pool.
	sock, acaP, err := GetACAClient()
	if err != nil {
		return err
	}
	defer sock.Close()

	req := &pb.ACAFetchAttrReq{
		Ts:        &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		ECert:     cert,
		Signature: nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		return err
	}

	var r, s *big.Int

	r, s, err = primitives.ECDSASignDirect(ecap.eca.priv, rawReq)

	if err != nil {
		return err
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := acaP.FetchAttributes(context.Background(), req)
	if err != nil {
		return err
	}

	if resp.Status != pb.ACAFetchAttrResp_FAILURE {
		return nil
	}
	return errors.New("Error fetching attributes.")
}

// CreateCertificatePair requests the creation of a new enrollment certificate pair by the ECA.
//
func (ecap *ECAP) CreateCertificatePair(ctx context.Context, in *pb.ECertCreateReq) (*pb.ECertCreateResp, error) {
	Trace.Println("gRPC ECAP:CreateCertificate")

	// validate token
	var tok, prev []byte
	var role, state int
	var enrollID string

	id := in.Id.Id
	err := ecap.eca.readUser(id).Scan(&role, &tok, &state, &prev, &enrollID)
	if err != nil {
		errMsg := "Identity lookup error: " + err.Error()
		Trace.Println(errMsg)
		return nil, errors.New(errMsg)
	}
	if !bytes.Equal(tok, in.Tok.Tok) {
		Trace.Printf("id or token mismatch: id=%s\n", id)
		return nil, errors.New("Identity or token does not match.")
	}

	ekey, err := x509.ParsePKIXPublicKey(in.Enc.Key)
	if err != nil {
		return nil, err
	}

	fetchResult := pb.FetchAttrsResult{Status: pb.FetchAttrsResult_SUCCESS, Msg: ""}
	switch {
	case state == 0:
		// initial request, create encryption challenge
		tok = []byte(randomString(12))

		_, err = ecap.eca.db.Exec("UPDATE Users SET token=?, state=?, key=? WHERE id=?", tok, 1, in.Enc.Key, id)
		if err != nil {
			Error.Println(err)
			return nil, err
		}

		spi := ecies.NewSPI()
		eciesKey, err := spi.NewPublicKey(nil, ekey.(*ecdsa.PublicKey))
		if err != nil {
			return nil, err
		}

		ecies, err := spi.NewAsymmetricCipherFromPublicKey(eciesKey)
		if err != nil {
			return nil, err
		}

		out, err := ecies.Process(tok)

		return &pb.ECertCreateResp{Certs: nil, Chain: nil, Pkchain: nil, Tok: &pb.Token{Tok: out}}, err

	case state == 1:
		// ensure that the same encryption key is signed that has been used for the challenge
		if subtle.ConstantTimeCompare(in.Enc.Key, prev) != 1 {
			return nil, errors.New("Encryption keys do not match.")
		}

		// validate request signature
		sig := in.Sig
		in.Sig = nil

		r, s := big.NewInt(0), big.NewInt(0)
		r.UnmarshalText(sig.R)
		s.UnmarshalText(sig.S)

		if in.Sign.Type != pb.CryptoType_ECDSA {
			return nil, errors.New("Unsupported (signing) key type.")
		}
		skey, err := x509.ParsePKIXPublicKey(in.Sign.Key)
		if err != nil {
			return nil, err
		}

		hash := primitives.NewHash()
		raw, _ := proto.Marshal(in)
		hash.Write(raw)
		if ecdsa.Verify(skey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
			return nil, errors.New("Signature verification failed.")
		}

		// create new certificate pair
		ts := time.Now().Add(-1 * time.Minute).UnixNano()

		spec := NewDefaultCertificateSpecWithCommonName(id, enrollID, skey.(*ecdsa.PublicKey), x509.KeyUsageDigitalSignature, pkix.Extension{Id: ECertSubjectRole, Critical: true, Value: []byte(strconv.Itoa(ecap.eca.readRole(id)))})
		sraw, err := ecap.eca.createCertificateFromSpec(spec, ts, nil, true)
		if err != nil {
			Error.Println(err)
			return nil, err
		}

		spec = NewDefaultCertificateSpecWithCommonName(id, enrollID, ekey.(*ecdsa.PublicKey), x509.KeyUsageDataEncipherment, pkix.Extension{Id: ECertSubjectRole, Critical: true, Value: []byte(strconv.Itoa(ecap.eca.readRole(id)))})
		eraw, err := ecap.eca.createCertificateFromSpec(spec, ts, nil, true)
		if err != nil {
			ecap.eca.db.Exec("DELETE FROM Certificates Where id=?", id)
			Error.Println(err)
			return nil, err
		}

		_, err = ecap.eca.db.Exec("UPDATE Users SET state=? WHERE id=?", 2, id)
		if err != nil {
			ecap.eca.db.Exec("DELETE FROM Certificates Where id=?", id)
			Error.Println(err)
			return nil, err
		}

		var obcECKey []byte
		if role == int(pb.Role_VALIDATOR) {
			obcECKey = ecap.eca.obcPriv
		} else {
			obcECKey = ecap.eca.obcPub
		}
		if role == int(pb.Role_CLIENT) {
			//Only client have to fetch attributes.
			if viper.GetBool("aca.enabled") {
				err = ecap.fetchAttributes(&pb.Cert{Cert: sraw})
				if err != nil {
					fetchResult = pb.FetchAttrsResult{Status: pb.FetchAttrsResult_FAILURE, Msg: err.Error()}

				}
			}
		}

		return &pb.ECertCreateResp{Certs: &pb.CertPair{Sign: sraw, Enc: eraw}, Chain: &pb.Token{Tok: ecap.eca.obcKey}, Pkchain: obcECKey, Tok: nil, FetchResult: &fetchResult}, nil
	}

	return nil, errors.New("Invalid (=expired) certificate creation token provided.")
}

// ReadCertificatePair reads an enrollment certificate pair from the ECA.
//
func (ecap *ECAP) ReadCertificatePair(ctx context.Context, in *pb.ECertReadReq) (*pb.CertPair, error) {
	Trace.Println("gRPC ECAP:ReadCertificate")

	rows, err := ecap.eca.readCertificates(in.Id.Id)
	defer rows.Close()

	var certs [][]byte
	if err == nil {
		for rows.Next() {
			var raw []byte
			err = rows.Scan(&raw)
			certs = append(certs, raw)
		}
		err = rows.Err()
	}

	return &pb.CertPair{Sign: certs[0], Enc: certs[1]}, err
}

// ReadCertificateByHash reads a single enrollment certificate by hash from the ECA.
//
func (ecap *ECAP) ReadCertificateByHash(ctx context.Context, hash *pb.Hash) (*pb.Cert, error) {
	Trace.Println("gRPC ECAP:ReadCertificateByHash")

	raw, err := ecap.eca.readCertificateByHash(hash.Hash)
	return &pb.Cert{Cert: raw}, err
}

// RevokeCertificatePair revokes a certificate pair from the ECA.  Not yet implemented.
//
func (ecap *ECAP) RevokeCertificatePair(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("gRPC ECAP:RevokeCertificate")

	return nil, errors.New("ECAP:RevokeCertificate method not (yet) implemented")
}

// RegisterUser registers a new user with the ECA.  If the user had been registered before
// an error is returned.
//
func (ecaa *ECAA) RegisterUser(ctx context.Context, in *pb.RegisterUserReq) (*pb.Token, error) {
	Trace.Println("gRPC ECAA:RegisterUser")

	// Check the signature
	err := ecaa.checkRegistrarSignature(in)
	if err != nil {
		return nil, err
	}

	// Register the user
	registrarID := in.Registrar.Id.Id
	in.Registrar.Id = nil
	registrar := pb.RegisterUserReq{Registrar: in.Registrar}
	json, err := json.Marshal(registrar)
	if err != nil {
		return nil, err
	}
	jsonStr := string(json)
	Trace.Println("gRPC ECAA:RegisterUser: json=" + jsonStr)
	tok, err := ecaa.eca.registerUser(in.Id.Id, in.Account, in.Affiliation, in.Role, registrarID, jsonStr)

	// Return the one-time password
	return &pb.Token{Tok: []byte(tok)}, err

}

func (ecaa *ECAA) checkRegistrarSignature(in *pb.RegisterUserReq) error {
	Trace.Println("ECAA.checkRegistrarSignature")

	// If no registrar was specified
	if in.Registrar == nil || in.Registrar.Id == nil || in.Registrar.Id.Id == "" {
		Trace.Println("gRPC ECAA:checkRegistrarSignature: no registrar was specified")
		return errors.New("no registrar was specified")
	}

	// Get the raw cert for the registrar
	registrar := in.Registrar.Id.Id
	raw, err := ecaa.eca.readCertificateByKeyUsage(registrar, x509.KeyUsageDigitalSignature)
	if err != nil {
		return err
	}

	// Parse the cert
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return err
	}

	// Remove the signature
	sig := in.Sig
	in.Sig = nil

	// Marshall the raw bytes
	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	hash := primitives.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)

	// Check the signature
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		// Signature verification failure
		Trace.Printf("ECAA.checkRegistrarSignature: failure for %s\n", registrar)
		return errors.New("Signature verification failed.")
	}

	// Signature verification was successful
	Trace.Printf("ECAA.checkRegistrarSignature: success for %s\n", registrar)
	return nil
}

// ReadUserSet returns a list of users matching the parameters set in the read request.
//
func (ecaa *ECAA) ReadUserSet(ctx context.Context, in *pb.ReadUserSetReq) (*pb.UserSet, error) {
	Trace.Println("gRPC ECAA:ReadUserSet")

	req := in.Req.Id
	if ecaa.eca.readRole(req)&int(pb.Role_AUDITOR) == 0 {
		return nil, errors.New("Access denied.")
	}

	raw, err := ecaa.eca.readCertificateByKeyUsage(req, x509.KeyUsageDigitalSignature)
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
		return nil, errors.New("Signature verification failed.")
	}

	rows, err := ecaa.eca.readUsers(int(in.Role))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*pb.User
	if err == nil {
		for rows.Next() {
			var id string
			var role int

			err = rows.Scan(&id, &role)
			users = append(users, &pb.User{Id: &pb.Identity{Id: id}, Role: pb.Role(role)})
		}
		err = rows.Err()
	}

	return &pb.UserSet{Users: users}, err
}

// RevokeCertificate revokes a certificate from the ECA.  Not yet implemented.
//
func (ecaa *ECAA) RevokeCertificate(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("gRPC ECAA:RevokeCertificate")

	return nil, errors.New("ECAA:RevokeCertificate method not (yet) implemented")
}

// PublishCRL requests the creation of a certificate revocation list from the ECA.  Not yet implemented.
//
func (ecaa *ECAA) PublishCRL(context.Context, *pb.ECertCRLReq) (*pb.CAStatus, error) {
	Trace.Println("gRPC ECAA:CreateCRL")

	return nil, errors.New("ECAA:PublishCRL method not (yet) implemented")
}

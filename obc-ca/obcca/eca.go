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
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	//	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"math/big"
	"strconv"
	"strings"
	"time"

	ecies "github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"

	"fmt"
	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"
	"github.com/openblockchain/obc-peer/openchain/crypto/conf"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
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

// NewECA sets up a new ECA.
//
func NewECA() *ECA {
	eca := &ECA{NewCA("eca"), nil, nil, nil}

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
			priv, err = ecdsa.GenerateKey(conf.GetDefaultCurve(), rand.Reader)
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

	// populate user table
	users := viper.GetStringMapString("eca.users")
	for id, flds := range users {
		vals := strings.Fields(flds)
		role, err := strconv.Atoi(vals[0])
		if err != nil {
			Panic.Panicln(err)
		}
		eca.registerUser(id, role, vals[1])
	}

	return eca
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
	Trace.Println("grpc ECAP:ReadCACertificate")

	return &pb.Cert{ecap.eca.raw}, nil
}

// CreateCertificatePair requests the creation of a new enrollment certificate pair by the ECA.
//
func (ecap *ECAP) CreateCertificatePair(ctx context.Context, in *pb.ECertCreateReq) (*pb.ECertCreateResp, error) {
	Trace.Println("grpc ECAP:CreateCertificate")

	// validate token
	var tok, prev []byte
	var role, state int

	id := in.Id.Id
	err := ecap.eca.readUser(id).Scan(&role, &tok, &state, &prev)
	if err != nil || !bytes.Equal(tok, in.Tok.Tok) {
		return nil, errors.New("identity or token do not match")
	}

	ekey, err := x509.ParsePKIXPublicKey(in.Enc.Key)
	if err != nil {
		return nil, err
	}

	switch {
	case state == 0:
		// initial request, create encryption challenge
		tok = []byte(randomString(12))

		_, err = ecap.eca.db.Exec("UPDATE Users SET token=?, state=?, key=? WHERE id=?", tok, 1, in.Enc.Key, id)
		if err != nil {
			Error.Println(err)
			return nil, err
		}

		//		out, err := rsa.EncryptPKCS1v15(rand.Reader, ekey.(*rsa.PublicKey), tok)
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
		return &pb.ECertCreateResp{nil, nil, nil, &pb.Token{out}}, err

	case state == 1:
		// ensure that the same encryption key is signed that has been used for the challenge
		if subtle.ConstantTimeCompare(in.Enc.Key, prev) != 1 {
			return nil, errors.New("encryption keys don't match")
		}

		// validate request signature
		sig := in.Sig
		in.Sig = nil

		r, s := big.NewInt(0), big.NewInt(0)
		r.UnmarshalText(sig.R)
		s.UnmarshalText(sig.S)

		if in.Sign.Type != pb.CryptoType_ECDSA {
			return nil, errors.New("unsupported key type")
		}
		skey, err := x509.ParsePKIXPublicKey(in.Sign.Key)
		if err != nil {
			return nil, err
		}

		hash := utils.NewHash()
		raw, _ := proto.Marshal(in)
		hash.Write(raw)
		if ecdsa.Verify(skey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
			return nil, errors.New("signature does not verify")
		}

		// create new certificate pair
		ts := time.Now().Add(-1 * time.Minute).UnixNano()

		sraw, err := ecap.eca.createCertificate(id, skey.(*ecdsa.PublicKey), x509.KeyUsageDigitalSignature, ts, nil, pkix.Extension{Id: ECertSubjectRole, Critical: true, Value: []byte(strconv.Itoa(ecap.eca.readRole(id)))})
		if err != nil {
			Error.Println(err)
			return nil, err
		}

		eraw, err := ecap.eca.createCertificate(id, ekey.(*ecdsa.PublicKey), x509.KeyUsageDataEncipherment, ts, nil, pkix.Extension{Id: ECertSubjectRole, Critical: true, Value: []byte(strconv.Itoa(ecap.eca.readRole(id)))})
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
			//if role&(int(pb.Role_VALIDATOR)|int(pb.Role_AUDITOR)) != 0 {
			obcECKey = ecap.eca.obcPriv
		} else {
			obcECKey = ecap.eca.obcPub
		}
		fmt.Printf("obcECKey % x\n", obcECKey)

		return &pb.ECertCreateResp{&pb.CertPair{sraw, eraw}, &pb.Token{ecap.eca.obcKey}, obcECKey, nil}, nil
	}

	return nil, errors.New("certificate creation token expired")
}

// ReadCertificatePair reads an enrollment certificate pair from the ECA.
//
func (ecap *ECAP) ReadCertificatePair(ctx context.Context, in *pb.ECertReadReq) (*pb.CertPair, error) {
	Trace.Println("grpc ECAP:ReadCertificate")

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

	return &pb.CertPair{certs[0], certs[1]}, err
}

// ReadCertificateByHash reads a single enrollment certificate by hash from the ECA.
//
func (ecap *ECAP) ReadCertificateByHash(ctx context.Context, hash *pb.Hash) (*pb.Cert, error) {
	Trace.Println("grpc ECAP:ReadCertificateByHash")

	raw, err := ecap.eca.readCertificateByHash(hash.Hash)
	return &pb.Cert{raw}, err
}

// RevokeCertificatePair revokes a certificate pair from the ECA.  Not yet implemented.
//
func (ecap *ECAP) RevokeCertificatePair(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RegisterUser registers a new user with the ECA.  If the user had been registered before
// an error is returned.
//
func (ecaa *ECAA) RegisterUser(ctx context.Context, in *pb.RegisterUserReq) (*pb.Token, error) {
	Trace.Println("grpc ECAA:RegisterUser")

	tok, err := ecaa.eca.registerUser(in.Id.Id, int(in.Role))

	return &pb.Token{[]byte(tok)}, err
}

// ReadUserSet returns a list of users matching the parameters set in the read request.
//
func (ecaa *ECAA) ReadUserSet(ctx context.Context, in *pb.ReadUserSetReq) (*pb.UserSet, error) {
	Trace.Println("grpc ECAA:ReadUserSet")

	req := in.Req.Id
	if ecaa.eca.readRole(req)&int(pb.Role_AUDITOR) == 0 {
		return nil, errors.New("access denied")
	}

	raw, err := ecaa.eca.readCertificate(req, x509.KeyUsageDigitalSignature)
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
			users = append(users, &pb.User{&pb.Identity{id}, pb.Role(role)})
		}
		err = rows.Err()
	}

	return &pb.UserSet{users}, err
}

// RevokeCertificate revokes a certificate from the ECA.  Not yet implemented.
//
func (ecaa *ECAA) RevokeCertificate(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAA:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// PublishCRL requests the creation of a certificate revocation list from the ECA.  Not yet implemented.
//
func (ecaa *ECAA) PublishCRL(context.Context, *pb.ECertCRLReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAA:CreateCRL")

	return nil, errors.New("not yet implemented")
}

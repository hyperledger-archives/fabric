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
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"
	"github.com/spf13/viper"
//	nacl "golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/sha3"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// ECA is the enrollment certificate authority.
//
type ECA struct {
	*CA
	obcKey          []byte
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
	var cooked string

	eca := &ECA{NewCA("eca"), nil}

	// read or create global symmetric encryption key
	raw, err := ioutil.ReadFile(eca.path+"/obc.key")
	if err != nil {
		rand := rand.Reader
		key := make([]byte, 32) // AES-256
		rand.Read(key)
		cooked = base64.StdEncoding.EncodeToString(key)

		err = ioutil.WriteFile(eca.path+"/obc.key", []byte(cooked), 0644)
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

	// populate user table
	users := viper.GetStringMapString("eca.users")
	for id, tok := range users {
		eca.registerUser(id, tok)
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
func (ecap *ECAP) CreateCertificatePair(ctx context.Context, req *pb.ECertCreateReq) (*pb.ECertCreateResp, error) {
	Trace.Println("grpc ECAP:CreateCertificate")

	// validate token
	var tok []byte
	var state int

	id := req.Id.Id
	err := ecap.eca.readToken(id).Scan(&tok, &state)
	if err != nil || !bytes.Equal(tok, req.Tok.Tok) {
		return nil, errors.New("identity or token do not match")
	}

	ekey, err := x509.ParsePKIXPublicKey(req.Enc.Key)
	if err != nil {
		return nil, err
	}

	switch {
	case state == 0:
		// initial request, create encryption challenge
		tok = []byte(randomString(12))

		_, err = ecap.eca.db.Exec("UPDATE Users SET token=?, state=? WHERE id=?", tok, 1, id)
		if err != nil {
			return nil, err
		}

		out, err := rsa.EncryptPKCS1v15(rand.Reader, ekey.(*rsa.PublicKey), tok)
		if err != nil {
			return nil, err
		}
		
		return &pb.ECertCreateResp{nil, nil, &pb.Token{out}}, nil

	case state == 1:
		// validate request signature
		sig := req.Sig
		req.Sig = nil

		r, s := big.NewInt(0), big.NewInt(0)
		r.UnmarshalText(sig.R)
		s.UnmarshalText(sig.S)

		if req.Sign.Type != pb.CryptoType_ECDSA {
			return nil, errors.New("unsupported key type")
		}
		skey, err := x509.ParsePKIXPublicKey(req.Sign.Key)
		if err != nil {
			return nil, err
		}

		hash := sha3.New384()
		raw, _ := proto.Marshal(req)
		hash.Write(raw)
		if ecdsa.Verify(skey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
			return nil, errors.New("signature does not verify")
		}

		// create new certificate pair
		ts := time.Now().UnixNano()

		sraw, err := ecap.eca.createCertificate(id, skey.(*ecdsa.PublicKey), x509.KeyUsageDigitalSignature, ts)
		if err != nil {
			Error.Println(err)
			return nil, err
		}

		eraw, err := ecap.eca.createCertificate(id, ekey.(*rsa.PublicKey), x509.KeyUsageDataEncipherment, ts)
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

		return &pb.ECertCreateResp{&pb.CertPair{sraw, eraw}, &pb.Token{ecap.eca.obcKey}, nil}, nil
	}

	return nil, errors.New("certificate creation token expired")
}

// ReadCertificatePair reads an enrollment certificate pair from the ECA.
//
func (ecap *ECAP) ReadCertificatePair(ctx context.Context, req *pb.ECertReadReq) (*pb.CertPair, error) {
	Trace.Println("grpc ECAP:ReadCertificate")

	rows, err := ecap.eca.readCertificates(req.Id.Id)
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
	if err != nil {
		return nil, err
	}

	return &pb.CertPair{certs[0], certs[1]}, nil
}

// ReadCertificateByHash reads a single enrollment certificate by hash from the ECA.
//
func (ecap *ECAP) ReadCertificateByHash(ctx context.Context, hash *pb.Hash) (*pb.Cert, error) {
	Trace.Println("grpc ECAP:ReadCertificateByHash")
	
	raw, err := ecap.eca.readCertificateByHash(hash.Hash)
	if err != nil {
		return nil, err
	}
	
	return &pb.Cert{raw}, nil
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
func (ecaa *ECAA) RegisterUser(ctx context.Context, id *pb.Identity) (*pb.Token, error) {
	Trace.Println("grpc ECAA:RegisterUser")

	tok, err := ecaa.eca.registerUser(id.Id)

	return &pb.Token{[]byte(tok)}, err
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

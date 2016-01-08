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
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"
	"github.com/spf13/viper"
	"golang.org/x/crypto/sha3"
	nacl "golang.org/x/crypto/nacl/box"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ECA is the enrollment certificate authority.
//
type ECA struct {
	*CA
	
	obcKey []byte
	encPub, encPriv []byte
	
	sockp, socka net.Listener
	srvp, srva   *grpc.Server
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

	eca := &ECA{NewCA("eca"), nil, nil, nil, nil, nil, nil, nil}

	// read or create global symmetric encryption key
	raw, err := ioutil.ReadFile(RootPath + "/obc.key")
	if err != nil {
		rand := rand.Reader
		key := make([]byte, 32) // AES-256
		rand.Read(key)
		cooked = base64.StdEncoding.EncodeToString(key)

		err = ioutil.WriteFile(RootPath+"/obc.key", []byte(cooked), 0644)
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

	// read or create ECA encryption key pair
	raw, err = ioutil.ReadFile(RootPath+"/eca.nacl")
	if err != nil {
		pub, priv, err := nacl.GenerateKey(rand.Reader)
		
		pair := make([]byte, 64)
		copy(pair[:32], pub[:])
		copy(pair[32:], priv[:])
		cooked = base64.StdEncoding.EncodeToString(pair)
		
		err = ioutil.WriteFile(RootPath+"/eca.nacl", []byte(cooked), 0644)
		if err != nil {
			Panic.Panicln(err)
		}
	} else {
		cooked = string(raw)
	}
	
	pair, err := base64.StdEncoding.DecodeString(cooked)
	eca.encPub = pair[:32]
	eca.encPriv = pair[32:]
	
	// populate user table
	users := viper.GetStringMapString("eca.users")
	for id, tok := range users {
		eca.registerUser(id, tok)
	}

	return eca
}

// Start starts the ECA.
//
func (eca *ECA) Start(wg *sync.WaitGroup) {
	var opts []grpc.ServerOption
	if viper.GetString("eca.tls.certfile") != "" {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("eca.tls.certfile"), viper.GetString("eca.tls.keyfile"))
		if err != nil {
			Panic.Panicln(err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	wg.Add(2)
	go eca.startECAP(wg, opts)
	go eca.startECAA(wg, opts)

	Info.Println("ECA started.")
}

// Stop stops the ECA.
//
func (eca *ECA) Stop() {
	eca.srvp.Stop()
	eca.srva.Stop()
}

func (eca *ECA) startECAP(wg *sync.WaitGroup, opts []grpc.ServerOption) {
	var err error

	eca.sockp, err = net.Listen("tcp", viper.GetString("ports.ecaP"))
	if err != nil {
		Panic.Panicln(err)
	}

	eca.srvp = grpc.NewServer(opts...)
	pb.RegisterECAPServer(eca.srvp, &ECAP{eca})
	eca.srvp.Serve(eca.sockp)

	_ = eca.sockp.Close()
	wg.Done()
}

func (eca *ECA) startECAA(wg *sync.WaitGroup, opts []grpc.ServerOption) {
	var err error

	eca.socka, err = net.Listen("tcp", viper.GetString("ports.ecaA"))
	if err != nil {
		Panic.Panicln(err)
	}

	eca.srva = grpc.NewServer(opts...)
	pb.RegisterECAAServer(eca.srva, &ECAA{eca})
	eca.srva.Serve(eca.socka)

	_ = eca.socka.Close()
	wg.Done()
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
	var tok string
	var state int

	id := req.Id.Id
	err := ecap.eca.readToken(id).Scan(&tok, &state)
	if err != nil || tok != req.Tok.Tok {
		return nil, errors.New("identity or token do not match")
	}

	switch {
	case state == 0:
		// initial request, create encryption challenge
		tok = randomString(12)

		_, err = ecap.eca.db.Exec("UPDATE Users SET token=?, state=? WHERE id=?", tok, 1, id)
		if err != nil {
			return nil, err
		}

		return &pb.ECertCreateResp{nil, &pb.Token{tok}}, nil

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
		ekey, err := x509.ParsePKIXPublicKey(req.Enc.Key)
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

		eraw, err := ecap.eca.createCertificate(id, ekey.(*ecdsa.PublicKey), x509.KeyUsageDataEncipherment, ts)
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

		return &pb.ECertCreateResp{&pb.CertPair{sraw, eraw}, nil}, nil
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

// RevokeCertificatePair revokes a certificate pair from the ECA.  Not yet implemented.
//
func (ecap *ECAP) RevokeCertificatePair(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RegisterUser registers a new user with the ECA.  If the user had been registered before all
// his/her certificates are deleted and the user is registered anew.
//
func (ecaa *ECAA) RegisterUser(ctx context.Context, id *pb.Identity) (*pb.Token, error) {
	Trace.Println("grpc ECAA:RegisterUser")

	tok, err := ecaa.eca.registerUser(id.Id)
	if err != nil {
		Error.Println(err)
	}

	return &pb.Token{tok}, err
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

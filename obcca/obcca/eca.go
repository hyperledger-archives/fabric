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
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obcca/protos"
	"github.com/spf13/viper"
	"golang.org/x/crypto/sha3"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ECA is the enrollment certificate authority.
//
type ECA struct {
	*CA
	
	sockp, socka net.Listener
	srvp, srva *grpc.Server
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
	eca := &ECA{NewCA("eca"), nil, nil, nil, nil}

	users := viper.GetStringMapString("eca.users")
	for id, pw := range users {
		eca.newUser(id, pw)
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

// CreateCertificate requests the creation of a new enrollment certificate by the ECA.
//
func (eca *ECAP) CreateCertificate(ctx context.Context, req *pb.ECertCreateReq) (*pb.Cert, error) {
	Trace.Println("grpc ECAP:CreateCertificate")

	id := req.Id.Id
	if pw, err := eca.eca.readPassword(id); err != nil || pw != req.Pw.Pw {
		Error.Println("identity or password do not match")
		return nil, errors.New("identity or password do not match")
	}

	sig := req.Sig
	req.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	raw := req.Pub.Key
	if req.Pub.Type != pb.CryptoType_ECDSA {
		Error.Println("unsupported key type")
		return nil, errors.New("unsupported key type")
	}
	pub, err := x509.ParsePKIXPublicKey(req.Pub.Key)
	if err != nil {
		Error.Println(err)
		return nil, err
	}

	hash := sha3.New384()
	raw, _ = proto.Marshal(req)
	hash.Write(raw)
	if ecdsa.Verify(pub.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		Error.Println("signature does not verify")
		return nil, errors.New("signature does not verify")
	}

	raw, err = eca.eca.readCertificate(id)
	if err != nil {
		if raw, err = eca.eca.newCertificate(id, pub.(*ecdsa.PublicKey), time.Now().UnixNano()); err != nil {
			Error.Println(err)
			return nil, err
		}
	}

	return &pb.Cert{raw}, nil
}

// ReadCertificate reads an enrollment certificate from the ECA.
//
func (eca *ECAP) ReadCertificate(ctx context.Context, req *pb.ECertReadReq) (*pb.Cert, error) {
	Trace.Println("grpc ECAP:ReadCertificate")

	var raw []byte
	var err error

	if req.Id.Id != "" {
		raw, err = eca.eca.readCertificate(req.Id.Id)
	} else {
		raw, err = eca.eca.readCertificateByHash(req.Hash)
	}
	if err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.Cert{raw}, nil
}

// RevokeCertificate revokes a certificate from the ECA.  Not yet implemented.
//
func (eca *ECAP) RevokeCertificate(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RegisterUser registers a new user with the ECA.
func (eca *ECAA) RegisterUser(ctx context.Context, id *pb.Identity) (*pb.Password, error) {
	Trace.Println("grpc ECAA:RegisterUser")

	pw, err := eca.eca.newUser(id.Id)
	if err != nil {
		Error.Println(err)
	}

	return &pb.Password{pw}, err
}

// RevokeCertificate revokes a certificate from the ECA.  Not yet implemented.
//
func (eca *ECAA) RevokeCertificate(context.Context, *pb.ECertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAA:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// CreateCRL requests the creation of a certificate revocation list from the ECA.  Not yet implemented.
//
func (eca *ECAA) CreateCRL(context.Context, *pb.ECertCRLReq) (*pb.CAStatus, error) {
	Trace.Println("grpc ECAA:CreateCRL")

	return nil, errors.New("not yet implemented")
}

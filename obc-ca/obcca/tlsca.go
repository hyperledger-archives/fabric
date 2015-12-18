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
	"crypto/x509"
	"github.com/op/go-logging"

	"math/big"
	"net"
	"time"
	"github.com/spf13/viper"
	"sync"
	
	"google.golang.org/grpc"

	"errors"
	"github.com/golang/protobuf/proto"
	
	"golang.org/x/crypto/sha3"
	
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	"crypto/ecdsa"
)

var logger = logging.MustGetLogger("tls_ca_pserver")


// TLSCA is the tls certificate authority.
//
type TLSCA struct {
	*CA
	sockp, socka net.Listener
	srvp, srva *grpc.Server
}

// TLSCAP serves the public GRPC interface of the TLSCA.
//
type TLSCAP struct {
	tlsca *TLSCA
}

// TLSCAA serves the administrator GRPC interface of the TLS.
//
type TLSCAA struct {
	tlsca *TLSCA
}

// NewTLSCA sets up a new TLSCA.
//
func NewTLSCA() *TLSCA {
	tlsca := &TLSCA{NewCA("tlsca"), nil, nil, nil, nil}
	
	return tlsca
}

// Start starts the TLSCA.
//
func (tlsca *TLSCA) Start(wg *sync.WaitGroup) {
	var opts []grpc.ServerOption
	if viper.GetString("tlsca.tls.certfile") != "" {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("tlsca.tls.certfile"), viper.GetString("tlsca.tls.keyfile"))
		if err != nil {
			Panic.Panicln(err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	wg.Add(2)
	go tlsca.startTLSCAP(opts)
	go tlsca.startTLSCAA(opts)

	Info.Println("TLSCA started.")
}

// Stop stops the TLSCA.
//
func (tlsca *TLSCA) Stop(wg *sync.WaitGroup) {
	tlsca.srvp.Stop()
	_ = tlsca.sockp.Close()
	wg.Done()

	tlsca.srva.Stop()
	_ = tlsca.socka.Close()
	wg.Done()
}

func (tlsca *TLSCA) startTLSCAP(opts []grpc.ServerOption) {
	var err error

	tlsca.sockp, err = net.Listen("tcp", viper.GetString("ports.tlscaP"))
	if err != nil {
		Panic.Panicln(err)
	}

	tlsca.srvp = grpc.NewServer(opts...)
	pb.RegisterTLSCAPServer(tlsca.srvp, &TLSCAP{tlsca})
	tlsca.srvp.Serve(tlsca.sockp)
}

func (tlsca *TLSCA) startTLSCAA(opts []grpc.ServerOption) {
	var err error

	tlsca.socka, err = net.Listen("tcp", viper.GetString("ports.tlscaA"))
	if err != nil {
		Panic.Panicln(err)
	}

	tlsca.srva = grpc.NewServer(opts...)
	pb.RegisterTLSCAAServer(tlsca.srva, &TLSCAA{tlsca})
	tlsca.srva.Serve(tlsca.socka)
}

// CreateCertificate requests the creation of a new enrollment certificate by the TLSCA.
//
func (tlscap *TLSCAP) CreateCertificate(ctx context.Context, req *pb.TLSCertCreateReq) (*pb.Cert, error) {
	Trace.Println("grpc TLSCA_P:CreateCertificate")

	id := req.Id.Id
	
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
	
	if raw, err = tlscap.tlsca.newCertificate(id, pub.(*ecdsa.PublicKey), time.Now().UnixNano()); err != nil {
		Error.Println(err)
		return nil, err
	}
	
	return &pb.Cert{raw}, nil
}

// ReadCertificate reads an enrollment certificate from the TLSCA.
//
func (tlscap *TLSCAP) ReadCertificate(ctx context.Context, req *pb.TLSCertReadReq) (*pb.Cert, error) {
	Trace.Println("grpc TLSCAP:ReadCertificate")

	var raw []byte
	var err error

	if req.Id.Id != "" {
		raw, err = tlscap.tlsca.readCertificate(req.Id.Id)
	} else {
		raw, err = tlscap.tlsca.readCertificateByHash(req.Hash)
	}
	if err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.Cert{raw}, nil
}

// RevokeCertificate revokes a certificate from the TLSCA.  Not yet implemented.
//
func (tlscap *TLSCAP) RevokeCertificate(context.Context, *pb.TLSCertRevokeReq) (*pb.CAStatus, error) {
	// TODO: We are not going to provide revocation support for now
	return nil, nil
}

// RevokeCertificate revokes a certificate from the TLSCA.  Not yet implemented.
//
func (tlscaa *TLSCAA) RevokeCertificate(context.Context, *pb.TLSCertRevokeReq) (*pb.CAStatus, error) {
	// TODO: We are not going to provide revocation support for now
	return nil, nil
}
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

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"
	"github.com/spf13/viper"
	"golang.org/x/crypto/sha3"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// TLSCA is the tls certificate authority.
//
type TLSCA struct {
	*CA
	eca     *ECA
	
	sockp, socka net.Listener
	srvp, srva   *grpc.Server
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
func NewTLSCA(eca *ECA) *TLSCA {
	tlsca := &TLSCA{NewCA("tlsca"), eca, nil, nil, nil, nil}

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

// ReadCACertificate reads the certificate of the TLSCA.
//
func (tlscap *TLSCAP) ReadCACertificate(ctx context.Context, in *pb.Empty) (*pb.Cert, error) {
	Trace.Println("grpc TLSCAP:ReadCACertificate")

	return &pb.Cert{tlscap.tlsca.raw}, nil
}

// CreateCertificate requests the creation of a new enrollment certificate by the TLSCA.
//
func (tlscap *TLSCAP) CreateCertificate(ctx context.Context, req *pb.TLSCertCreateReq) (*pb.TLSCertCreateResp, error) {
	Trace.Println("grpc TLSCAP:CreateCertificate")

	id := req.Id.Id
	raw, err := tlscap.tlsca.eca.readCertificate(id, x509.KeyUsageDigitalSignature)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}

	sig := req.Sig
	req.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	raw = req.Pub.Key
	if req.Pub.Type != pb.CryptoType_ECDSA {
		return nil, errors.New("unsupported key type")
	}
	pub, err := x509.ParsePKIXPublicKey(req.Pub.Key)
	if err != nil {
		return nil, err
	}

	hash := sha3.New384()
	raw, _ = proto.Marshal(req)
	hash.Write(raw)
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	if raw, err = tlscap.tlsca.createCertificate(id, pub.(*ecdsa.PublicKey), x509.KeyUsageKeyAgreement, req.Ts.Seconds); err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.TLSCertCreateResp{&pb.Cert{raw}}, nil
}

// ReadCertificate reads an enrollment certificate from the TLSCA.
//
func (tlscap *TLSCAP) ReadCertificate(ctx context.Context, req *pb.TLSCertReadReq) (*pb.Cert, error) {
	Trace.Println("grpc TLSCAP:ReadCertificate")

	raw, err := tlscap.tlsca.readCertificate(req.Id.Id, x509.KeyUsageKeyAgreement)
	if err != nil {
		return nil, err
	}

	return &pb.Cert{raw}, nil
}

// RevokeCertificate revokes a certificate from the TLSCA.  Not yet implemented.
//
func (tlscap *TLSCAP) RevokeCertificate(context.Context, *pb.TLSCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TLSCAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificate revokes a certificate from the TLSCA.  Not yet implemented.
//
func (tlscaa *TLSCAA) RevokeCertificate(context.Context, *pb.TLSCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TLSCAA:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

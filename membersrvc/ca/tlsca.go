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
	"crypto/x509"
	"errors"
	"math/big"
	"database/sql"


	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// TLSCA is the tls certificate authority.
//
type TLSCA struct {
	*CA
	eca *ECA
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

func initializeTLSCATables(db *sql.DB) error { 
	return initializeCommonTables(db)
}

// NewTLSCA sets up a new TLSCA.
//
func NewTLSCA(eca *ECA) *TLSCA {
	tlsca := &TLSCA{NewCA("tlsca", initializeTLSCATables), eca}

	return tlsca
}

// Start starts the TLSCA.
//
func (tlsca *TLSCA) Start(srv *grpc.Server) {
	tlsca.startTLSCAP(srv)
	tlsca.startTLSCAA(srv)

	Info.Println("TLSCA started.")
}

func (tlsca *TLSCA) startTLSCAP(srv *grpc.Server) {
	pb.RegisterTLSCAPServer(srv, &TLSCAP{tlsca})
}

func (tlsca *TLSCA) startTLSCAA(srv *grpc.Server) {
	pb.RegisterTLSCAAServer(srv, &TLSCAA{tlsca})
}

// ReadCACertificate reads the certificate of the TLSCA.
//
func (tlscap *TLSCAP) ReadCACertificate(ctx context.Context, in *pb.Empty) (*pb.Cert, error) {
	Trace.Println("grpc TLSCAP:ReadCACertificate")

	return &pb.Cert{tlscap.tlsca.raw}, nil
}

// CreateCertificate requests the creation of a new enrollment certificate by the TLSCA.
//
func (tlscap *TLSCAP) CreateCertificate(ctx context.Context, in *pb.TLSCertCreateReq) (*pb.TLSCertCreateResp, error) {
	Trace.Println("grpc TLSCAP:CreateCertificate")

	id := in.Id.Id

	sig := in.Sig
	in.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	raw := in.Pub.Key
	if in.Pub.Type != pb.CryptoType_ECDSA {
		return nil, errors.New("unsupported key type")
	}
	pub, err := x509.ParsePKIXPublicKey(in.Pub.Key)
	if err != nil {
		return nil, err
	}

	hash := primitives.NewHash()
	raw, _ = proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(pub.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		return nil, errors.New("signature does not verify")
	}

	if raw, err = tlscap.tlsca.createCertificate(id, pub.(*ecdsa.PublicKey), x509.KeyUsageDigitalSignature, in.Ts.Seconds, nil); err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.TLSCertCreateResp{&pb.Cert{raw}, &pb.Cert{tlscap.tlsca.raw}}, nil
}

// ReadCertificate reads an enrollment certificate from the TLSCA.
//
func (tlscap *TLSCAP) ReadCertificate(ctx context.Context, in *pb.TLSCertReadReq) (*pb.Cert, error) {
	Trace.Println("grpc TLSCAP:ReadCertificate")

	raw, err := tlscap.tlsca.readCertificate(in.Id.Id, x509.KeyUsageKeyAgreement)
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

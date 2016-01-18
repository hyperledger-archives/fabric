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
	"crypto/hmac"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"sync"
	"strconv"

	"golang.org/x/crypto/sha3"

	"github.com/golang/protobuf/proto"
	pb "github.com/openblockchain/obc-peer/obc-ca/protos"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
)

var (
	// TCertEncTCertIndex is the ASN1 object identifier of the TCert index.
	//
	TCertEncTCertIndex = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 7}
)

// TCA is the transaction certificate authority.
//
type TCA struct {
	*CA
	eca     *ECA
	hmacKey []byte

	rand io.Reader
		
	sockp, socka net.Listener
	srvp, srva *grpc.Server
}

// TCAP serves the public GRPC interface of the TCA.
//
type TCAP struct {
	tca *TCA
}

// TCAA serves the administrator GRPC interface of the TCA.
//
type TCAA struct {
	tca *TCA
}

// NewTCA sets up a new TCA.
//
func NewTCA(eca *ECA) *TCA {
	var cooked string

	tca := &TCA{NewCA("tca"), eca, nil, rand.Reader, nil, nil, nil, nil}
	
	raw, err := ioutil.ReadFile(RootPath + "/tca.hmac")
	if err != nil {
		rand := rand.Reader
		key := make([]byte, 49)
		rand.Read(key)
		cooked = base64.StdEncoding.EncodeToString(key)

		err = ioutil.WriteFile(RootPath+"/tca.hmac", []byte(cooked), 0644)
		if err != nil {
			Panic.Panicln(err)
		}
	} else {
		cooked = string(raw)
	}

	tca.hmacKey, err = base64.StdEncoding.DecodeString(cooked)
	if err != nil {
		Panic.Panicln(err)
	}

	return tca
}

// Start starts the TCA.
//
func (tca *TCA) Start(wg *sync.WaitGroup) {
	var opts []grpc.ServerOption
	if viper.GetString("tca.tls.certfile") != "" {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("tca.tls.certfile"), viper.GetString("tca.tls.keyfile"))
		if err != nil {
			Panic.Panicln(err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	wg.Add(2)
	go tca.startTCAP(wg, opts)
	go tca.startTCAA(wg, opts)
	
	tca.startValidityPeriodUpdate()
	
	Info.Println("TCA started.")
}

func (tca *TCA) startValidityPeriodUpdate(){
	if validityPeriodUpdateEnabled() {
		go updateValidityPeriod()
	}
}

// Stop stops the TCA.
//
func (tca *TCA) Stop() {
	tca.srvp.Stop()
	tca.srva.Stop()
}

func (tca *TCA) startTCAP(wg *sync.WaitGroup, opts []grpc.ServerOption) {
	var err error
	
	tca.sockp, err = net.Listen("tcp", viper.GetString("ports.tcaP"))
	if err != nil {
		Panic.Panicln(err)
	}

	tca.srvp = grpc.NewServer(opts...)
	pb.RegisterTCAPServer(tca.srvp, &TCAP{tca})
	tca.srvp.Serve(tca.sockp)

	_ = tca.sockp.Close()
	wg.Done()
}

func (tca *TCA) startTCAA(wg *sync.WaitGroup, opts []grpc.ServerOption) {
	var err error
	
	tca.socka, err = net.Listen("tcp", viper.GetString("ports.tcaA"))
	if err != nil {
		Panic.Panicln(err)
	}

	tca.srva = grpc.NewServer(opts...)
	pb.RegisterTCAAServer(tca.srva, &TCAA{tca})
	tca.srva.Serve(tca.socka)

	_ = tca.socka.Close()
	wg.Done()
}

// CreateCertificate requests the creation of a new transaction certificate by the TCA.
//
func (tcap *TCAP) CreateCertificate(ctx context.Context, req *pb.TCertCreateReq) (*pb.Cert, error) {
	Trace.Println("grpc TCAP:CreateCertificate")

	id := req.Id.Id
	raw, err := tcap.tca.eca.readCertificate(id)
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
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		Error.Println("signature does not verify")
		return nil, errors.New("signature does not verify")
	}

	if raw, err = tcap.tca.newCertificate(id, pub.(*ecdsa.PublicKey), req.Ts.Seconds); err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.Cert{raw}, nil
}

// CreateCertificateSet requests the creation of a new transaction certificate set by the TCA.
//
func (tcap *TCAP) CreateCertificateSet(ctx context.Context, req *pb.TCertCreateSetReq) (*pb.CertSet, error) {
	Trace.Println("grpc TCAP:CreateCertificateSet")

	id := req.Id.Id
	raw, err := tcap.tca.eca.readCertificate(id)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	pub := cert.PublicKey.(*ecdsa.PublicKey)

	sig := req.Sig
	req.Sig = nil

	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(sig.R)
	s.UnmarshalText(sig.S)

	hash := sha3.New384()
	raw, _ = proto.Marshal(req)
	hash.Write(raw)
	if ecdsa.Verify(pub, hash.Sum(nil), r, s) == false {
		Error.Println("signature does not verify")
		return nil, errors.New("signature does not verify")
	}

	nonce := make([]byte, 16) // 8 bytes rand, 8 bytes timestamp
	tcap.tca.rand.Read(nonce[:8])
	binary.LittleEndian.PutUint64(nonce[8:], uint64(req.Ts.Seconds))
	
	mac := hmac.New(sha3.New384, tcap.tca.hmacKey)
	raw, _ = x509.MarshalPKIXPublicKey(pub)
	mac.Write(raw)
	kdfKey := mac.Sum(nil)

	num := int(req.Num)
	if num == 0 {
		num = 1
	}
	var set [][]byte

	for i := 0; i < num; i++ {
		tidx := []byte(strconv.Itoa(i))
		tidx = append(tidx[:], nonce[:]...)
		mac = hmac.New(sha3.New384, kdfKey)
		mac.Write([]byte{1})
		extKey := mac.Sum(nil)[:32]
		mac = hmac.New(sha3.New384, kdfKey)
		mac.Write([]byte{2})
		mac = hmac.New(sha3.New384, mac.Sum(nil))
		mac.Write(tidx)

		one := new(big.Int).SetInt64(1)
		k := new(big.Int).SetBytes(mac.Sum(nil))
		k.Mod(k, new(big.Int).Sub(pub.Curve.Params().N, one))
		k.Add(k, one)

		tmpX, tmpY := pub.ScalarBaseMult(k.Bytes())
		txX, txY := pub.Curve.Add(pub.X, pub.Y, tmpX, tmpY)
		txPub := ecdsa.PublicKey{Curve: pub.Curve, X: txX, Y: txY}

		ext, err := CBCEncrypt(extKey, tidx)
		if err != nil {
			return nil, err
		}
		if raw, err = tcap.tca.newCertificate(id, &txPub, req.Ts.Seconds, pkix.Extension{Id: TCertEncTCertIndex, Critical: true, Value: ext}); err != nil {
			Error.Println(err)
			return nil, err
		}
		set = append(set, raw)
	}

	return &pb.CertSet{kdfKey, set}, nil
}

// ReadCertificate reads a transaction certificate from the TCA.
//
func (tcap *TCAP) ReadCertificate(ctx context.Context, req *pb.TCertReadReq) (*pb.Cert, error) {
	Trace.Println("grpc TCAP:ReadCertificate")

	id := req.Id.Id
	if id == "tca-root" {
		raw, err := tcap.tca.readCertificate(id)
		return &pb.Cert{raw}, err
	}

	raw, err := tcap.tca.eca.readCertificate(id)
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

	hash := sha3.New384()
	raw, _ = proto.Marshal(req)
	hash.Write(raw)
	if ecdsa.Verify(cert.PublicKey.(*ecdsa.PublicKey), hash.Sum(nil), r, s) == false {
		Error.Println("signature does not verify")
		return nil, errors.New("signature does not verify")
	}

	raw, err = tcap.tca.readCertificate(id, req.Ts.Seconds)
	if err != nil {
		Error.Println(err)
		return nil, err
	}

	return &pb.Cert{raw}, nil
}

// ReadCertificateSet reads a transaction certificate set from the TCA.  Not yet implemented.
//
func (tcap *TCAP) ReadCertificateSet(context.Context, *pb.TCertReadSetReq) (*pb.CertSet, error) {
	Trace.Println("grpc TCAP:ReadCertificateSet")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificate revokes a certificate from the TCA.  Not yet implemented.
//
func (tcap *TCAP) RevokeCertificate(context.Context, *pb.TCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAP:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificateSet revokes a certificate set from the TCA.  Not yet implemented.
//
func (tcap *TCAP) RevokeCertificateSet(context.Context, *pb.TCertRevokeSetReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAP:RevokeCertificateSet")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificate revokes a certificate from the TCA.  Not yet implemented.
//
func (tcaa *TCAA) RevokeCertificate(context.Context, *pb.TCertRevokeReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:RevokeCertificate")

	return nil, errors.New("not yet implemented")
}

// RevokeCertificateSet revokes a certificate set from the TCA.  Not yet implemented.
//
func (tcaa *TCAA) RevokeCertificateSet(context.Context, *pb.TCertRevokeSetReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:RevokeCertificateSet")

	return nil, errors.New("not yet implemented")
}

// CreateCRL requests the creation of a certificate revocation list from the TCA.  Not yet implemented.
//
func (tcaa *TCAA) CreateCRL(context.Context, *pb.TCertCRLReq) (*pb.CAStatus, error) {
	Trace.Println("grpc TCAA:CreateCRL")

	return nil, errors.New("not yet implemented")
}

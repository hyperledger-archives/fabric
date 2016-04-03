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

package ca

import (
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"google/protobuf"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"github.com/hyperledger/fabric/core/util"

	_ "fmt"

	membersrvc "github.com/hyperledger/fabric/membersrvc/protos"
)

var (
	eca_s   *ECA
	tlsca_s *TLSCA
	srv     *grpc.Server
)

func TestTLS(t *testing.T) {
	// Skipping test for now, this is just to try tls connections
	t.Skip()

	go startTLSCA(t)

	time.Sleep(time.Second * 10)

	requestTLSCertificate(t)

	stopTLSCA(t)
}

func startTLSCA(t *testing.T) {
	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca_s = NewECA()
	tlsca_s = NewTLSCA(eca_s)

	var opts []grpc.ServerOption
	creds, err := credentials.NewServerTLSFromFile(viper.GetString("server.tls.certfile"), viper.GetString("server.tls.keyfile"))
	if err != nil {
		t.Logf("Failed creating credentials for TLS-CA service: %s", err)
		t.Fail()
	}

	opts = []grpc.ServerOption{grpc.Creds(creds)}

	srv = grpc.NewServer(opts...)

	eca_s.Start(srv)
	tlsca_s.Start(srv)

	sock, err := net.Listen("tcp", viper.GetString("server.port"))
	if err != nil {
		t.Logf("Failed to start TLS-CA service: %s", err)
		t.Fail()
	}

	srv.Serve(sock)
}

func requestTLSCertificate(t *testing.T) {
	var opts []grpc.DialOption

	creds, err := credentials.NewClientTLSFromFile(viper.GetString("server.tls.certfile"), "tlsca")
	if err != nil {
		t.Logf("Failed creating credentials for TLS-CA client: %s", err)
		t.Fail()
	}

	opts = append(opts, grpc.WithTransportCredentials(creds))
	sockP, err := grpc.Dial(viper.GetString("peer.pki.tlsca.paddr"), opts...)
	if err != nil {
		t.Logf("Failed dialing in: %s", err)
		t.Fail()
	}

	defer sockP.Close()

	tlscaP := membersrvc.NewTLSCAPClient(sockP)

	// Prepare the request
	id := "peer"
	priv, err := utils.NewECDSAKey()

	if err != nil {
		t.Logf("Failed generating key: %s", err)
		t.Fail()
	}

	uuid := util.GenerateUUID()

	pubraw, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}

	req := &membersrvc.TLSCertCreateReq{
		&timestamp,
		&membersrvc.Identity{Id: id + "-" + uuid},
		&membersrvc.PublicKey{
			Type: membersrvc.CryptoType_ECDSA,
			Key:  pubraw,
		}, nil}

	rawreq, _ := proto.Marshal(req)
	r, s, err := ecdsa.Sign(rand.Reader, priv, utils.Hash(rawreq))

	if err != nil {
		t.Logf("Failed signing the request: %s", err)
		t.Fail()
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &membersrvc.Signature{membersrvc.CryptoType_ECDSA, R, S}

	resp, err := tlscaP.CreateCertificate(context.Background(), req)
	if err != nil {
		t.Logf("Failed requesting tls certificate: %s", err)
		t.Fail()
	}

	storePrivateKeyInClear("tls_peer.priv", priv, t)
	storeCert("tls_peer.cert", resp.Cert.Cert, t)
	storeCert("tls_peer.ca", resp.RootCert.Cert, t)
}

func stopTLSCA(t *testing.T) {
	srv.Stop()
}

func storePrivateKeyInClear(alias string, privateKey interface{}, t *testing.T) {
	rawKey, err := utils.PrivateKeyToPEM(privateKey, nil)
	if err != nil {
		t.Logf("Failed converting private key to PEM [%s]: [%s]", alias, err)
		t.Fail()
	}

	err = ioutil.WriteFile(filepath.Join(".membersrvc/", alias), rawKey, 0700)
	if err != nil {
		t.Logf("Failed storing private key [%s]: [%s]", alias, err)
		t.Fail()
	}
}

func storeCert(alias string, der []byte, t *testing.T) {
	err := ioutil.WriteFile(filepath.Join(".membersrvc/", alias), utils.DERCertToPEM(der), 0700)
	if err != nil {
		t.Logf("Failed storing certificate [%s]: [%s]", alias, err)
		t.Fail()
	}
}

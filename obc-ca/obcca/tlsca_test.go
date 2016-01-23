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
	"golang.org/x/net/context"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	_ "fmt"
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"
)

var (
	eca_s   *ECA
	tlsca_s *TLSCA
	srv     *grpc.Server
)

func TestTLS(t *testing.T) {
	go startTLSCA()

	time.Sleep(time.Second * 30)

	requestTLSCertificate()

	stopTLSCA()
}

func startTLSCA() {
	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca_s = NewECA()
	tlsca_s = NewTLSCA(eca_s)

	var opts []grpc.ServerOption
	creds, err := credentials.NewServerTLSFromFile(viper.GetString("server.tls.certfile"), viper.GetString("server.tls.keyfile"))
	if err != nil {
		panic("Failed creating credentials for TLS-CA service: " + err.Error())
	}

	opts = []grpc.ServerOption{grpc.Creds(creds)}

	srv = grpc.NewServer(opts...)

	eca_s.Start(srv)
	tlsca_s.Start(srv)

	sock, err := net.Listen("tcp", viper.GetString("server.port"))
	if err != nil {
		panic(err)
	}

	srv.Serve(sock)
}

func requestTLSCertificate() {
	var opts []grpc.DialOption

	creds, err := credentials.NewClientTLSFromFile(viper.GetString("server.tls.certfile"), "OBC")
	if err != nil {
		grpclog.Fatalf("Failed to create TLS credentials %v", err)
	}

	opts = append(opts, grpc.WithTransportCredentials(creds))
	sockP, err := grpc.Dial(viper.GetString("peer.pki.tlsca.paddr"), opts...)
	if err != nil {
		grpclog.Fatalf("Failed dialing in: %s", err)
	}

	defer sockP.Close()

	tlscaP := obcca.NewTLSCAPClient(sockP)

	_, err = tlscaP.CreateCertificate(context.Background(), nil)
	if err != nil {
		grpclog.Fatalf("Failed requesting tls certificate: %s", err)
	}
}

func stopTLSCA() {
	srv.Stop()
}
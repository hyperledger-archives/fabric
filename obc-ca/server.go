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

package main

import (
	//	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"runtime"

	"fmt"
	"github.com/openblockchain/obc-peer/obc-ca/obcca"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	viper.AutomaticEnv()
	viper.SetConfigName("obcca")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	var iotrace, ioinfo, iowarning, ioerror, iopanic io.Writer
	if obcca.GetConfigInt("logging.trace") == 1 {
		iotrace = os.Stdout
	} else {
		iotrace = ioutil.Discard
	}
	if obcca.GetConfigInt("logging.info") == 1 {
		ioinfo = os.Stdout
	} else {
		ioinfo = ioutil.Discard
	}
	if obcca.GetConfigInt("logging.warning") == 1 {
		iowarning = os.Stdout
	} else {
		iowarning = ioutil.Discard
	}
	if obcca.GetConfigInt("logging.error") == 1 {
		ioerror = os.Stderr
	} else {
		ioerror = ioutil.Discard
	}
	if obcca.GetConfigInt("logging.panic") == 1 {
		iopanic = os.Stdout
	} else {
		iopanic = ioutil.Discard
	}

	// Init the crypto layer
	if err := crypto.Init(); err != nil {
		panic(fmt.Errorf("Failed initializing the crypto layer [%s]%", err))
	}

	obcca.LogInit(iotrace, ioinfo, iowarning, ioerror, iopanic)
	obcca.Info.Println("CA Server (" + viper.GetString("server.version") + ")")

	eca := obcca.NewECA()
	defer eca.Close()

	tca := obcca.NewTCA(eca)
	defer tca.Close()

	tlsca := obcca.NewTLSCA(eca)
	defer tlsca.Close()

	runtime.GOMAXPROCS(obcca.GetConfigInt("server.gomaxprocs"))

	var opts []grpc.ServerOption
	if viper.GetString("server.tls.certfile") != "" {
		creds, err := credentials.NewServerTLSFromFile(viper.GetString("server.tls.certfile"), viper.GetString("server.tls.keyfile"))
		if err != nil {
			panic(err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	srv := grpc.NewServer(opts...)

	eca.Start(srv)
	tca.Start(srv)
	tlsca.Start(srv)

	sock, err := net.Listen("tcp", obcca.GetConfigString("server.port"))
	if err != nil {
		panic(err)
	}

	srv.Serve(sock)

	sock.Close()
}

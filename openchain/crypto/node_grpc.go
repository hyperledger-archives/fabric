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

package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func (node *nodeImpl) initTLS() error {
	node.debug("Initiliazing TLS...")

	if node.conf.isTLSEnabled() {
		pem, err := node.ks.loadExternalCert(node.conf.getTLSCACertsExternalPath())
		if err != nil {
			node.error("Failed loading TLSCA certificates chain [%s].", err.Error())

			return err
		}

		node.tlsCertPool = x509.NewCertPool()
		ok := node.tlsCertPool.AppendCertsFromPEM(pem)
		if !ok {
			node.error("Failed appending TLSCA certificates chain.")

			return errors.New("Failed appending TLSCA certificates chain.")
		}
		node.debug("Initiliazing TLS...Done")
	} else {
		node.debug("Initiliazing TLS...Disabled!!!")
	}

	return nil
}

func (node *nodeImpl) getClientConn(address string, serverName string) (*grpc.ClientConn, error) {
	node.debug("Dial to addr:[%s], with serverName:[%s]...", address, serverName)

	var conn *grpc.ClientConn
	var err error

	if node.conf.isTLSEnabled() {
		node.debug("TLS enabled...")

		// setup tls options
		var opts []grpc.DialOption
		config := tls.Config{
			InsecureSkipVerify: false,
			RootCAs:            node.tlsCertPool,
			ServerName:         serverName,
		}
		if node.conf.isTLSClientAuthEnabled() {

		}

		creds := credentials.NewTLS(&config)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		opts = append(opts, grpc.WithTimeout(time.Second*3))

		conn, err = grpc.Dial(address, opts...)
	} else {
		node.debug("TLS disabled...")

		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())
		opts = append(opts, grpc.WithTimeout(time.Second*3))

		conn, err = grpc.Dial(address, opts...)
	}

	if err != nil {
		node.error("Failed dailing in [%s].", err.Error())

		return nil, err
	}

	node.debug("Dial to addr:[%s], with serverName:[%s]...done!", address, serverName)

	return conn, nil
}

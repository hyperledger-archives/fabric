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
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"

	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"google/protobuf"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func (node *nodeImpl) initTLS() error {
	node.log.Debug("Initiliazing TLS...")

	if node.conf.isTLSEnabled() {
		pem, err := node.ks.loadExternalCert(node.conf.getTLSCACertsExternalPath())
		if err != nil {
			node.log.Error("Failed loading TLSCA certificates chain [%s].", err.Error())

			return err
		}

		node.tlsCertPool = x509.NewCertPool()
		ok := node.tlsCertPool.AppendCertsFromPEM(pem)
		if !ok {
			node.log.Error("Failed appending TLSCA certificates chain.")

			return errors.New("Failed appending TLSCA certificates chain.")
		}
		node.log.Debug("Initiliazing TLS...Done")
	} else {
		node.log.Debug("Initiliazing TLS...Disabled!!!")
	}

	return nil
}

func (node *nodeImpl) retrieveTLSCertificate(id, affiliation string) error {
	key, tlsCertRaw, err := node.getTLSCertificateFromTLSCA(id, affiliation)
	if err != nil {
		node.log.Error("Failed getting tls certificate [id=%s] %s", id, err)

		return err
	}
	node.log.Debug("TLS Cert [%s]", utils.EncodeBase64(tlsCertRaw))

	node.log.Info("Storing TLS key and certificate for user [%s]...", id)

	// Store tls key.
	if err := node.ks.storePrivateKeyInClear(node.conf.getTLSKeyFilename(), key); err != nil {
		node.log.Error("Failed storing tls key [id=%s]: %s", id, err)
		return err
	}

	// Store tls cert
	if err := node.ks.storeCert(node.conf.getTLSCertFilename(), tlsCertRaw); err != nil {
		node.log.Error("Failed storing tls certificate [id=%s]: %s", id, err)
		return err
	}

	return nil
}

func (node *nodeImpl) loadTLSCertificate() error {
	node.log.Debug("Loading tls certificate...")

	cert, _, err := node.ks.loadCertX509AndDer(node.conf.getTLSCertFilename())
	if err != nil {
		node.log.Error("Failed parsing tls certificate [%s].", err.Error())

		return err
	}
	node.tlsCert = cert

	return nil
}

func (node *nodeImpl) loadTLSCACertsChain() error {
	if node.conf.isTLSEnabled() {
		node.log.Debug("Loading TLSCA certificates chain...")

		pem, err := node.ks.loadExternalCert(node.conf.getTLSCACertsExternalPath())
		if err != nil {
			node.log.Error("Failed loading TLSCA certificates chain [%s].", err.Error())

			return err
		}

		ok := node.tlsCertPool.AppendCertsFromPEM(pem)
		if !ok {
			node.log.Error("Failed appending TLSCA certificates chain.")

			return errors.New("Failed appending TLSCA certificates chain.")
		}

		node.log.Debug("Loading TLSCA certificates chain...done")

		return nil
	} else {
		node.log.Debug("TLS is disabled!!!")
	}

	return nil
}

func (node *nodeImpl) getTLSCertificateFromTLSCA(id, affiliation string) (interface{}, []byte, error) {
	node.log.Info("getTLSCertificate...")

	priv, err := utils.NewECDSAKey()

	if err != nil {
		node.log.Error("Failed generating key: %s", err)

		return nil, nil, err
	}

	uuid := util.GenerateUUID()

	// Prepare the request
	pubraw, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}

	req := &obcca.TLSCertCreateReq{
		&timestamp,
		&obcca.Identity{Id: id + "-" + uuid},
		&obcca.PublicKey{
			Type: obcca.CryptoType_ECDSA,
			Key:  pubraw,
		}, nil}
	rawreq, _ := proto.Marshal(req)
	r, s, err := ecdsa.Sign(rand.Reader, priv, utils.Hash(rawreq))
	if err != nil {
		panic(err)
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	pbCert, err := node.callTLSCACreateCertificate(context.Background(), req)
	if err != nil {
		node.log.Error("Failed requesting tls certificate: %s", err)

		return nil, nil, err
	}

	node.log.Info("Verifing tls certificate...")

	tlsCert, err := utils.DERToX509Certificate(pbCert.Cert.Cert)
	certPK := tlsCert.PublicKey.(*ecdsa.PublicKey)
	utils.VerifySignCapability(priv, certPK)

	node.log.Info("Verifing tls certificate...done!")

	return priv, pbCert.Cert.Cert, nil
}

func (node *nodeImpl) getClientConn(address string, serverName string) (*grpc.ClientConn, error) {
	node.log.Debug("Getting Client Connection to [%s]...", serverName)

	var conn *grpc.ClientConn
	var err error

	if node.conf.isTLSEnabled() {
		node.log.Debug("TLS enabled...")

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
		node.log.Debug("TLS disabled...")

		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())
		opts = append(opts, grpc.WithTimeout(time.Second*3))

		conn, err = grpc.Dial(address, opts...)
	}

	if err != nil {
		node.log.Error("Failed dailing in [%s].", err.Error())

		return nil, err
	}

	node.log.Debug("Getting Client Connection to [%s]...done", serverName)

	return conn, nil
}

func (node *nodeImpl) getTLSCAClient() (*grpc.ClientConn, obcca.TLSCAPClient, error) {
	node.log.Debug("Getting TLSCA client...")

	conn, err := node.getClientConn(node.conf.getTLSCAPAddr(), node.conf.getTLSCAServerName())
	if err != nil {
		node.log.Error("Failed getting client connection: [%s]", err)
	}

	client := obcca.NewTLSCAPClient(conn)

	node.log.Debug("Getting TLSCA client...done")

	return conn, client, nil
}

func (node *nodeImpl) callTLSCACreateCertificate(ctx context.Context, in *obcca.TLSCertCreateReq, opts ...grpc.CallOption) (*obcca.TLSCertCreateResp, error) {
	conn, tlscaP, err := node.getTLSCAClient()
	if err != nil {
		node.log.Error("Failed dialing in: %s", err)

		return nil, err
	}
	defer conn.Close()

	resp, err := tlscaP.CreateCertificate(ctx, in, opts...)
	if err != nil {
		node.log.Error("Failed requesting tls certificate: %s", err)

		return nil, err
	}

	return resp, nil
}

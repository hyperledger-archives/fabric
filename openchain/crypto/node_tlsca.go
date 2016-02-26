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
	"crypto/x509"
	"errors"
	"google/protobuf"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func (node *nodeImpl) retrieveTLSCertificate(id, affiliation string) error {
	key, tlsCertRaw, err := node.getTLSCertificateFromTLSCA(id, affiliation)
	if err != nil {
		node.error("Failed getting tls certificate [id=%s] %s", id, err)

		return err
	}
	node.debug("TLS Cert [% x]", tlsCertRaw)

	node.debug("Storing TLS key and certificate for user [%s]...", id)

	// Store tls key.
	if err := node.ks.storePrivateKeyInClear(node.conf.getTLSKeyFilename(), key); err != nil {
		node.error("Failed storing tls key [id=%s]: %s", id, err)
		return err
	}

	// Store tls cert
	if err := node.ks.storeCert(node.conf.getTLSCertFilename(), tlsCertRaw); err != nil {
		node.error("Failed storing tls certificate [id=%s]: %s", id, err)
		return err
	}

	return nil
}

func (node *nodeImpl) loadTLSCertificate() error {
	node.debug("Loading tls certificate...")

	cert, _, err := node.ks.loadCertX509AndDer(node.conf.getTLSCertFilename())
	if err != nil {
		node.error("Failed parsing tls certificate [%s].", err.Error())

		return err
	}
	node.tlsCert = cert

	return nil
}

func (node *nodeImpl) loadTLSCACertsChain() error {
	if node.conf.isTLSEnabled() {
		node.debug("Loading TLSCA certificates chain...")

		pem, err := node.ks.loadExternalCert(node.conf.getTLSCACertsExternalPath())
		if err != nil {
			node.error("Failed loading TLSCA certificates chain [%s].", err.Error())

			return err
		}

		ok := node.tlsCertPool.AppendCertsFromPEM(pem)
		if !ok {
			node.error("Failed appending TLSCA certificates chain.")

			return errors.New("Failed appending TLSCA certificates chain.")
		}

		node.debug("Loading TLSCA certificates chain...done")

	} else {
		node.debug("TLS is disabled!!!")
	}

	return nil
}

func (node *nodeImpl) getTLSCertificateFromTLSCA(id, affiliation string) (interface{}, []byte, error) {
	node.debug("getTLSCertificate...")

	priv, err := utils.NewECDSAKey()

	if err != nil {
		node.error("Failed generating key: %s", err)

		return nil, nil, err
	}

	uuid := util.GenerateUUID()

	// Prepare the request
	pubraw, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	now := time.Now()
	timestamp := google_protobuf.Timestamp{Seconds: int64(now.Second()), Nanos: int32(now.Nanosecond())}

	req := &obcca.TLSCertCreateReq{
		Ts: &timestamp,
		Id: &obcca.Identity{Id: id + "-" + uuid},
		Pub: &obcca.PublicKey{
			Type: obcca.CryptoType_ECDSA,
			Key:  pubraw,
		}, Sig: nil}
	rawreq, _ := proto.Marshal(req)
	r, s, err := ecdsa.Sign(rand.Reader, priv, utils.Hash(rawreq))
	if err != nil {
		panic(err)
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{Type: obcca.CryptoType_ECDSA, R: R, S: S}

	pbCert, err := node.callTLSCACreateCertificate(context.Background(), req)
	if err != nil {
		node.error("Failed requesting tls certificate: %s", err)

		return nil, nil, err
	}

	node.debug("Verifing tls certificate...")

	tlsCert, err := utils.DERToX509Certificate(pbCert.Cert.Cert)
	certPK := tlsCert.PublicKey.(*ecdsa.PublicKey)
	utils.VerifySignCapability(priv, certPK)

	node.debug("Verifing tls certificate...done!")

	return priv, pbCert.Cert.Cert, nil
}

func (node *nodeImpl) getTLSCAClient() (*grpc.ClientConn, obcca.TLSCAPClient, error) {
	node.debug("Getting TLSCA client...")

	conn, err := node.getClientConn(node.conf.getTLSCAPAddr(), node.conf.getTLSCAServerName())
	if err != nil {
		node.error("Failed getting client connection: [%s]", err)
	}

	client := obcca.NewTLSCAPClient(conn)

	node.debug("Getting TLSCA client...done")

	return conn, client, nil
}

func (node *nodeImpl) callTLSCACreateCertificate(ctx context.Context, in *obcca.TLSCertCreateReq, opts ...grpc.CallOption) (*obcca.TLSCertCreateResp, error) {
	conn, tlscaP, err := node.getTLSCAClient()
	if err != nil {
		node.error("Failed dialing in: %s", err)

		return nil, err
	}
	defer conn.Close()

	resp, err := tlscaP.CreateCertificate(ctx, in, opts...)
	if err != nil {
		node.error("Failed requesting tls certificate: %s", err)

		return nil, err
	}

	return resp, nil
}

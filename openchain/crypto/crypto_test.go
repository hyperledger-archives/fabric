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
	obc "github.com/openblockchain/obc-peer/protos"

	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"crypto/rand"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/obc-ca/obcca"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type createTxFunc func(t *testing.T) (*obc.Transaction, *obc.Transaction, error)

var (
	validator Peer

	peer Peer

	deployer Client
	invoker  Client

	server *grpc.Server
	eca    *obcca.ECA
	tca    *obcca.TCA
	tlsca  *obcca.TLSCA

	deployTxCreators  []createTxFunc
	executeTxCreators []createTxFunc
	queryTxCreators   []createTxFunc

	ksPwd = []byte("This is a very very very long pw")
)

func TestMain(m *testing.M) {
	setup()

	// Init PKI
	initPKI()
	go startPKI()
	defer cleanup()

	// Init clients
	err := initClients()
	if err != nil {
		fmt.Printf("Failed initializing clients [%s]\n", err)
		panic(fmt.Errorf("Failed initializing clients [%s].", err))
	}

	// Init peer
	err = initPeers()
	if err != nil {
		fmt.Printf("Failed initializing peers [%s]\n", err)
		panic(fmt.Errorf("Failed initializing peers [%s].", err))
	}

	// Init validators
	err = initValidators()
	if err != nil {
		fmt.Printf("Failed initializing validators [%s]\n", err)
		panic(fmt.Errorf("Failed initializing validators [%s].", err))
	}

	viper.Set("pki.validity-period.update", "false")
	viper.Set("validator.validity-period.verification", "false")

	if err != nil {
		fmt.Printf("Failed initializing ledger [%s]\n", err.Error())
		panic(fmt.Errorf("Failed initializing ledger [%s].", err.Error()))
	}

	ret := m.Run()

	cleanup()

	os.Exit(ret)
}

func TestParallelInitClose(t *testing.T) {
	// TODO: complete this
	conf := utils.NodeConfiguration{Type: "client", Name: "userthread"}
	RegisterClient(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())

	done := make(chan bool)

	n := 10
	for i := 0; i < n; i++ {
		go func() {
			for i := 0; i < 5; i++ {
				client, err := InitClient(conf.Name, nil)
				if err != nil {
					t.Log("Init failed")
				}

				cis := &obc.ChaincodeInvocationSpec{
					ChaincodeSpec: &obc.ChaincodeSpec{
						Type:                 obc.ChaincodeSpec_GOLANG,
						ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
						CtorMsg:              nil,
						ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
					},
				}
				for i := 0; i < 20; i++ {
					uuid := util.GenerateUUID()
					client.NewChaincodeExecute(cis, uuid)
				}

				err = CloseClient(client)
				if err != nil {
					t.Log("Close failed")
				}
			}
			done <- true

		}()
	}
	for i := 0; i < n; i++ {
		log.Info("Waiting")
		<-done
		log.Info("+1")
	}
	log.Info("Test Finished!")
	//
}

func TestRegistrationSameEnrollIDDifferentRole(t *testing.T) {
	conf := utils.NodeConfiguration{Type: "client", Name: "TestRegistrationSameEnrollIDDifferentRole"}
	if err := RegisterClient(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD()); err != nil {
		t.Fatalf("Failed client registration [%s]", err)
	}

	if err := RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD()); err == nil {
		t.Fatalf("Reusing the same enrollment id must be forbidden", err)
	}

	if err := RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD()); err == nil {
		t.Fatalf("Reusing the same enrollment id must be forbidden", err)
	}
}

func TestInitialization(t *testing.T) {
	// Init fake client
	client, err := InitClient("", nil)
	if err == nil || client != nil {
		t.Fatal("Init should fail")
	}
	err = CloseClient(client)
	if err == nil {
		t.Fatal("Close should fail")
	}

	// Init fake peer
	peer, err := InitPeer("", nil)
	if err == nil || peer != nil {
		t.Fatal("Init should fail")
	}
	err = ClosePeer(peer)
	if err == nil {
		t.Fatal("Close should fail")
	}

	// Init fake validator
	validator, err := InitValidator("", nil)
	if err == nil || validator != nil {
		t.Fatal("Init should fail")
	}
	err = CloseValidator(validator)
	if err == nil {
		t.Fatal("Close should fail")
	}
}

func TestClientDeployTransaction(t *testing.T) {
	for i, createTx := range deployTxCreators {
		t.Logf("TestClientDeployTransaction with [%d]\n", i)

		_, tx, err := createTx(t)

		if err != nil {
			t.Fatalf("Failed creating deploy transaction [%s].", err)
		}

		if tx == nil {
			t.Fatalf("Result must be different from nil")
		}

		// Check transaction. For test purposes only
		err = deployer.(*clientImpl).checkTransaction(tx)
		if err != nil {
			t.Fatalf("Failed checking transaction [%s].", err)
		}
	}
}

func TestClientExecuteTransaction(t *testing.T) {
	for i, createTx := range executeTxCreators {
		t.Logf("TestClientExecuteTransaction with [%d]\n", i)

		_, tx, err := createTx(t)

		if err != nil {
			t.Fatalf("Failed creating deploy transaction [%s].", err)
		}

		if tx == nil {
			t.Fatalf("Result must be different from nil")
		}

		// Check transaction. For test purposes only
		err = invoker.(*clientImpl).checkTransaction(tx)
		if err != nil {
			t.Fatalf("Failed checking transaction [%s].", err)
		}
	}
}

func TestClientQueryTransaction(t *testing.T) {
	for i, createTx := range queryTxCreators {
		t.Logf("TestClientQueryTransaction with [%d]\n", i)

		_, tx, err := createTx(t)

		if err != nil {
			t.Fatalf("Failed creating deploy transaction [%s].", err)
		}

		if tx == nil {
			t.Fatalf("Result must be different from nil")
		}

		// Check transaction. For test purposes only
		err = invoker.(*clientImpl).checkTransaction(tx)
		if err != nil {
			t.Fatalf("Failed checking transaction [%s].", err)
		}
	}
}

func TestClientMultiExecuteTransaction(t *testing.T) {
	for i := 0; i < 24; i++ {
		_, tx, err := createConfidentialExecuteTransaction(t)

		if err != nil {
			t.Fatalf("Failed creating execute transaction [%s].", err)
		}

		if tx == nil {
			t.Fatalf("Result must be different from nil")
		}

		// Check transaction. For test purposes only
		err = invoker.(*clientImpl).checkTransaction(tx)
		if err != nil {
			t.Fatalf("Failed checking transaction [%s].", err)
		}
	}
}

func TestClientGetTCertHandlerNext(t *testing.T) {
	handler, err := deployer.GetTCertificateHandlerNext()

	if err != nil {
		t.Fatalf("Failed getting handler: [%s]", err)
	}
	if handler == nil {
		t.Fatalf("Handler should be different from nil")
	}

	certDER := handler.GetCertificate()

	if certDER == nil {
		t.Fatalf("Cert should be different from nil")
	}
	if len(certDER) == 0 {
		t.Fatalf("Cert should have length > 0")
	}
}

func TestClientGetTCertHandlerFromDER(t *testing.T) {
	handler, err := deployer.GetTCertificateHandlerNext()
	if err != nil {
		t.Fatalf("Failed getting handler: [%s]", err)
	}

	handler2, err := deployer.GetTCertificateHandlerFromDER(handler.GetCertificate())
	if err != nil {
		t.Fatalf("Failed getting tcert: [%s]", err)
	}
	if handler == nil {
		t.Fatalf("Handler should be different from nil")
	}
	tCertDER := handler2.GetCertificate()
	if tCertDER == nil {
		t.Fatalf("TCert should be different from nil")
	}
	if len(tCertDER) == 0 {
		t.Fatalf("TCert should have length > 0")
	}

	if !reflect.DeepEqual(handler.GetCertificate(), tCertDER) {
		t.Fatalf("TCerts must be the same")
	}
}

func TestClientTCertHandlerSign(t *testing.T) {
	handlerDeployer, err := deployer.GetTCertificateHandlerNext()
	if err != nil {
		t.Fatalf("Failed getting handler: [%s]", err)
	}

	msg := []byte("Hello World!!!")
	signature, err := handlerDeployer.Sign(msg)
	if err != nil {
		t.Fatalf("Failed getting tcert: [%s]", err)
	}
	if signature == nil || len(signature) == 0 {
		t.Fatalf("Failed getting non-nil signature")
	}

	err = handlerDeployer.Verify(signature, msg)
	if err != nil {
		t.Fatalf("Failed verifying signature: [%s]", err)
	}

	// Check that invoker (another party) can verify the signature
	handlerInvoker, err := invoker.GetTCertificateHandlerFromDER(handlerDeployer.GetCertificate())
	if err != nil {
		t.Fatalf("Failed getting tcert: [%s]", err)
	}

	err = handlerInvoker.Verify(signature, msg)
	if err != nil {
		t.Fatalf("Failed verifying signature: [%s]", err)
	}

	// Check that invoker cannot sign using a tcert obtained by the deployer
	signature, err = handlerInvoker.Sign(msg)
	if err == nil {
		t.Fatalf("Bob should not be able to use Alice's tcert to sign")
	}
	if signature != nil {
		t.Fatalf("Signature should be nil")
	}
}

func TestClientGetEnrollmentCertHandler(t *testing.T) {
	handler, err := deployer.GetEnrollmentCertificateHandler()

	if err != nil {
		t.Fatalf("Failed getting handler: [%s]", err)
	}
	if handler == nil {
		t.Fatalf("Handler should be different from nil")
	}

	certDER := handler.GetCertificate()

	if certDER == nil {
		t.Fatalf("Cert should be different from nil")
	}
	if len(certDER) == 0 {
		t.Fatalf("Cert should have length > 0")
	}
}

func TestClientGetEnrollmentCertHandlerSign(t *testing.T) {
	handlerDeployer, err := deployer.GetEnrollmentCertificateHandler()
	if err != nil {
		t.Fatalf("Failed getting handler: [%s]", err)
	}

	msg := []byte("Hello World!!!")
	signature, err := handlerDeployer.Sign(msg)
	if err != nil {
		t.Fatalf("Failed getting tcert: [%s]", err)
	}
	if signature == nil || len(signature) == 0 {
		t.Fatalf("Failed getting non-nil signature")
	}

	err = handlerDeployer.Verify(signature, msg)
	if err != nil {
		t.Fatalf("Failed verifying signature: [%s]", err)
	}

	// Check that invoker (another party) can verify the signature
	handlerInvoker, err := invoker.GetEnrollmentCertificateHandler()
	if err != nil {
		t.Fatalf("Failed getting tcert: [%s]", err)
	}

	err = handlerInvoker.Verify(signature, msg)
	if err == nil {
		t.Fatalf("Failed verifying signature: [%s]", err)
	}

}

func TestPeerID(t *testing.T) {
	// Verify that any id modification doesn't change
	id := peer.GetID()

	if id == nil {
		t.Fatalf("Id is nil.")
	}

	if len(id) == 0 {
		t.Fatalf("Id length is zero.")
	}

	id[0] = id[0] + 1
	id2 := peer.GetID()
	if id2[0] == id[0] {
		t.Fatalf("Invariant not respected.")
	}
}

func TestPeerDeployTransaction(t *testing.T) {
	for i, createTx := range deployTxCreators {
		t.Logf("TestPeerDeployTransaction with [%d]\n", i)

		_, tx, err := createTx(t)
		if err != nil {
			t.Fatalf("TransactionPreValidation: failed creating transaction [%s].", err)
		}

		res, err := peer.TransactionPreValidation(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		res, err = peer.TransactionPreExecution(tx)
		if err != utils.ErrNotImplemented {
			t.Fatalf("Error must be ErrNotImplemented [%s].", err)
		}
		if res != nil {
			t.Fatalf("Result must nil")
		}
	}
}

func TestPeerExecuteTransaction(t *testing.T) {
	for i, createTx := range executeTxCreators {
		t.Logf("TestPeerExecuteTransaction with [%d]\n", i)

		_, tx, err := createTx(t)
		if err != nil {
			t.Fatalf("TransactionPreValidation: failed creating transaction [%s].", err)
		}

		res, err := peer.TransactionPreValidation(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		res, err = peer.TransactionPreExecution(tx)
		if err != utils.ErrNotImplemented {
			t.Fatalf("Error must be ErrNotImplemented [%s].", err)
		}
		if res != nil {
			t.Fatalf("Result must nil")
		}
	}
}

func TestPeerQueryTransaction(t *testing.T) {
	for i, createTx := range queryTxCreators {
		t.Logf("TestPeerQueryTransaction with [%d]\n", i)

		_, tx, err := createTx(t)
		if err != nil {
			t.Fatalf("Failed creating query transaction [%s].", err)
		}

		res, err := peer.TransactionPreValidation(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		res, err = peer.TransactionPreExecution(tx)
		if err != utils.ErrNotImplemented {
			t.Fatalf("Error must be ErrNotImplemented [%s].", err)
		}
		if res != nil {
			t.Fatalf("Result must nil")
		}
	}
}

func TestPeerStateEncryptor(t *testing.T) {
	_, deployTx, err := createConfidentialDeployTransaction(t)
	if err != nil {
		t.Fatalf("Failed creating deploy transaction [%s].", err)
	}
	_, invokeTxOne, err := createConfidentialExecuteTransaction(t)
	if err != nil {
		t.Fatalf("Failed creating invoke transaction [%s].", err)
	}

	res, err := peer.GetStateEncryptor(deployTx, invokeTxOne)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented [%s].", err)
	}
	if res != nil {
		t.Fatalf("Result must be nil")
	}
}

func TestPeerSignVerify(t *testing.T) {
	msg := []byte("Hello World!!!")
	signature, err := peer.Sign(msg)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented [%s].", err)
	}
	if signature != nil {
		t.Fatalf("Result must be nil")
	}

	err = peer.Verify(validator.GetID(), signature, msg)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented [%s].", err)
	}
}

func TestValidatorID(t *testing.T) {
	// Verify that any id modification doesn't change
	id := validator.GetID()

	if id == nil {
		t.Fatalf("Id is nil.")
	}

	if len(id) == 0 {
		t.Fatalf("Id length is zero.")
	}

	id[0] = id[0] + 1
	id2 := validator.GetID()
	if id2[0] == id[0] {
		t.Fatalf("Invariant not respected.")
	}
}

func TestValidatorDeployTransaction(t *testing.T) {
	for i, createTx := range deployTxCreators {
		t.Logf("TestValidatorDeployTransaction with [%d]\n", i)

		otx, tx, err := createTx(t)
		if err != nil {
			t.Fatalf("Failed creating deploy transaction [%s].", err)
		}

		res, err := validator.TransactionPreValidation(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		res, err = validator.TransactionPreExecution(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		if tx.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
			if reflect.DeepEqual(res, tx) {
				t.Fatalf("Src and Dest Transaction should be different after PreExecution")
			}
			if err := isEqual(otx, res); err != nil {
				t.Fatalf("Decrypted transaction differs from the original: [%s]", err)
			}
		}
	}
}

func TestValidatorExecuteTransaction(t *testing.T) {
	for i, createTx := range executeTxCreators {
		t.Logf("TestValidatorExecuteTransaction with [%d]\n", i)

		otx, tx, err := createTx(t)
		if err != nil {
			t.Fatalf("Failed creating execute transaction [%s].", err)
		}

		res, err := validator.TransactionPreValidation(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		res, err = validator.TransactionPreExecution(tx)
		if err != nil {
			t.Fatalf("Error must be nil [%s].", err)
		}
		if res == nil {
			t.Fatalf("Result must be diffrent from nil")
		}

		if tx.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
			if reflect.DeepEqual(res, tx) {
				t.Fatalf("Src and Dest Transaction should be different after PreExecution")
			}
			if err := isEqual(otx, res); err != nil {
				t.Fatalf("Decrypted transaction differs from the original: [%s]", err)
			}
		}
	}
}

func TestValidatorQueryTransaction(t *testing.T) {
	for i, createTx := range queryTxCreators {
		t.Logf("TestValidatorConfidentialQueryTransaction with [%d]\n", i)

		_, deployTx, err := deployTxCreators[i](t)
		if err != nil {
			t.Fatalf("Failed creating deploy transaction [%s].", err)
		}
		_, invokeTxOne, err := executeTxCreators[i](t)
		if err != nil {
			t.Fatalf("Failed creating invoke transaction [%s].", err)
		}
		_, invokeTxTwo, err := executeTxCreators[i](t)
		if err != nil {
			t.Fatalf("Failed creating invoke transaction [%s].", err)
		}
		otx, queryTx, err := createTx(t)
		if err != nil {
			t.Fatalf("Failed creating query transaction [%s].", err)
		}

		if queryTx.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {

			// Transactions must be PreExecuted by the validators before getting the StateEncryptor
			if _, err := validator.TransactionPreValidation(deployTx); err != nil {
				t.Fatalf("Failed pre-validating deploty transaction [%s].", err)
			}
			if deployTx, err = validator.TransactionPreExecution(deployTx); err != nil {
				t.Fatalf("Failed pre-executing deploty transaction [%s].", err)
			}
			if _, err := validator.TransactionPreValidation(invokeTxOne); err != nil {
				t.Fatalf("Failed pre-validating exec1 transaction [%s].", err)
			}
			if invokeTxOne, err = validator.TransactionPreExecution(invokeTxOne); err != nil {
				t.Fatalf("Failed pre-executing exec1 transaction [%s].", err)
			}
			if _, err := validator.TransactionPreValidation(invokeTxTwo); err != nil {
				t.Fatalf("Failed pre-validating exec2 transaction [%s].", err)
			}
			if invokeTxTwo, err = validator.TransactionPreExecution(invokeTxTwo); err != nil {
				t.Fatalf("Failed pre-executing exec2 transaction [%s].", err)
			}
			if _, err := validator.TransactionPreValidation(queryTx); err != nil {
				t.Fatalf("Failed pre-validating query transaction [%s].", err)
			}
			if queryTx, err = validator.TransactionPreExecution(queryTx); err != nil {
				t.Fatalf("Failed pre-executing query transaction [%s].", err)
			}
			if err := isEqual(otx, queryTx); err != nil {
				t.Fatalf("Decrypted transaction differs from the original: [%s]", err)
			}

			// First invokeTx
			seOne, err := validator.GetStateEncryptor(deployTx, invokeTxOne)
			if err != nil {
				t.Fatalf("Failed creating state encryptor [%s].", err)
			}
			pt := []byte("Hello World")
			aCt, err := seOne.Encrypt(pt)
			if err != nil {
				t.Fatalf("Failed encrypting state [%s].", err)
			}
			aPt, err := seOne.Decrypt(aCt)
			if err != nil {
				t.Fatalf("Failed decrypting state [%s].", err)
			}
			if !bytes.Equal(pt, aPt) {
				t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
			}

			// Second invokeTx
			seTwo, err := validator.GetStateEncryptor(deployTx, invokeTxTwo)
			if err != nil {
				t.Fatalf("Failed creating state encryptor [%s].", err)
			}
			aPt2, err := seTwo.Decrypt(aCt)
			if err != nil {
				t.Fatalf("Failed decrypting state [%s].", err)
			}
			if !bytes.Equal(pt, aPt2) {
				t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
			}

			// queryTx
			seThree, err := validator.GetStateEncryptor(deployTx, queryTx)
			ctQ, err := seThree.Encrypt(aPt2)
			if err != nil {
				t.Fatalf("Failed encrypting query result [%s].", err)
			}

			aPt3, err := invoker.DecryptQueryResult(queryTx, ctQ)
			if err != nil {
				t.Fatalf("Failed decrypting query result [%s].", err)
			}
			if !bytes.Equal(aPt2, aPt3) {
				t.Fatalf("Failed decrypting query result [%s != %s]: %s", string(aPt2), string(aPt3), err)
			}

		}
	}
}

func TestValidatorStateEncryptor(t *testing.T) {
	_, deployTx, err := createConfidentialDeployTransaction(t)
	if err != nil {
		t.Fatalf("Failed creating deploy transaction [%s]", err)
	}
	_, invokeTxOne, err := createConfidentialExecuteTransaction(t)
	if err != nil {
		t.Fatalf("Failed creating invoke transaction [%s]", err)
	}
	_, invokeTxTwo, err := createConfidentialExecuteTransaction(t)
	if err != nil {
		t.Fatalf("Failed creating invoke transaction [%s]", err)
	}

	// Transactions must be PreExecuted by the validators before getting the StateEncryptor
	if _, err := validator.TransactionPreValidation(deployTx); err != nil {
		t.Fatalf("Failed pre-validating deploty transaction [%s].", err)
	}
	if deployTx, err = validator.TransactionPreExecution(deployTx); err != nil {
		t.Fatalf("Failed pre-validating deploty transaction [%s].", err)
	}
	if _, err := validator.TransactionPreValidation(invokeTxOne); err != nil {
		t.Fatalf("Failed pre-validating exec1 transaction [%s].", err)
	}
	if invokeTxOne, err = validator.TransactionPreExecution(invokeTxOne); err != nil {
		t.Fatalf("Failed pre-validating exec1 transaction [%s].", err)
	}
	if _, err := validator.TransactionPreValidation(invokeTxTwo); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction [%s].", err)
	}
	if invokeTxTwo, err = validator.TransactionPreExecution(invokeTxTwo); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction [%s].", err)
	}

	seOne, err := validator.GetStateEncryptor(deployTx, invokeTxOne)
	if err != nil {
		t.Fatalf("Failed creating state encryptor [%s].", err)
	}
	pt := []byte("Hello World")
	aCt, err := seOne.Encrypt(pt)
	if err != nil {
		t.Fatalf("Failed encrypting state [%s].", err)
	}
	aPt, err := seOne.Decrypt(aCt)
	if err != nil {
		t.Fatalf("Failed decrypting state [%s].", err)
	}
	if !bytes.Equal(pt, aPt) {
		t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
	}

	seTwo, err := validator.GetStateEncryptor(deployTx, invokeTxTwo)
	if err != nil {
		t.Fatalf("Failed creating state encryptor [%s].", err)
	}
	aPt2, err := seTwo.Decrypt(aCt)
	if err != nil {
		t.Fatalf("Failed decrypting state [%s].", err)
	}
	if !bytes.Equal(pt, aPt2) {
		t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
	}

}

func TestValidatorSignVerify(t *testing.T) {
	msg := []byte("Hello World!!!")
	signature, err := validator.Sign(msg)
	if err != nil {
		t.Fatalf("TestSign: failed generating signature [%s].", err)
	}

	err = validator.Verify(validator.GetID(), signature, msg)
	if err != nil {
		t.Fatalf("TestSign: failed validating signature [%s].", err)
	}
}

func TestValidatorVerify(t *testing.T) {
	msg := []byte("Hello World!!!")
	signature, err := validator.Sign(msg)
	if err != nil {
		t.Fatalf("Failed generating signature [%s].", err)
	}

	err = validator.Verify(nil, signature, msg)
	if err == nil {
		t.Fatalf("Verify should fail when given an empty id.", err)
	}

	err = validator.Verify(msg, signature, msg)
	if err == nil {
		t.Fatalf("Verify should fail when given an invalid id.", err)
	}

	err = validator.Verify(validator.GetID(), nil, msg)
	if err == nil {
		t.Fatalf("Verify should fail when given an invalid signature.", err)
	}

	err = validator.Verify(validator.GetID(), msg, msg)
	if err == nil {
		t.Fatalf("Verify should fail when given an invalid signature.", err)
	}

	err = validator.Verify(validator.GetID(), signature, nil)
	if err == nil {
		t.Fatalf("Verify should fail when given an invalid messahe.", err)
	}
}

func BenchmarkTransactionCreation(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()
	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}
	invoker.GetTCertificateHandlerNext()

	for i := 0; i < b.N; i++ {
		uuid := util.GenerateUUID()
		b.StartTimer()
		invoker.NewChaincodeExecute(cis, uuid)
		b.StopTimer()
	}
}

func BenchmarkTransactionValidation(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, tx, _ := createConfidentialTCertHExecuteTransaction(nil)

		b.StartTimer()
		validator.TransactionPreValidation(tx)
		validator.TransactionPreExecution(tx)
		b.StopTimer()
	}
}

func BenchmarkSign(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	//b.Logf("#iterations %d\n", b.N)
	signKey, _ := utils.NewECDSAKey()
	hash := make([]byte, 48)

	for i := 0; i < b.N; i++ {
		rand.Read(hash)
		b.StartTimer()
		utils.ECDSASign(signKey, hash)
		b.StopTimer()
	}
}

func BenchmarkVerify(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	//b.Logf("#iterations %d\n", b.N)
	signKey, _ := utils.NewECDSAKey()
	verKey := signKey.PublicKey
	hash := make([]byte, 48)

	for i := 0; i < b.N; i++ {
		rand.Read(hash)
		sigma, _ := utils.ECDSASign(signKey, hash)
		b.StartTimer()
		utils.ECDSAVerify(&verKey, hash, sigma)
		b.StopTimer()
	}
}

func setup() {
	// Conf
	viper.SetConfigName("crypto_test") // name of config file (without extension)
	viper.AddConfigPath(".")           // path to look for the config file in
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file [%s] \n", err))
	}
	// Set Default properties
	viper.Set("peer.fileSystemPath", filepath.Join(os.TempDir(), "obc-crypto-tests", "peers"))
	viper.Set("server.rootpath", filepath.Join(os.TempDir(), "obc-crypto-tests", "ca"))
	viper.Set("peer.pki.tls.rootcert.file", filepath.Join(os.TempDir(),
		"obc-crypto-tests", "ca", "tlsca.cert"))

	// Logging
	var formatter = logging.MustStringFormatter(
		`%{color}[%{module}] %{shortfunc} [%{shortfile}] -> %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	logging.SetFormatter(formatter)

	// TX creators
	deployTxCreators = []createTxFunc{
		createPublicDeployTransaction,
		createConfidentialDeployTransaction,
		createConfidentialTCertHDeployTransaction,
		createConfidentialECertHDeployTransaction,
	}
	executeTxCreators = []createTxFunc{
		createPublicExecuteTransaction,
		createConfidentialExecuteTransaction,
		createConfidentialTCertHExecuteTransaction,
		createConfidentialECertHExecuteTransaction,
	}
	queryTxCreators = []createTxFunc{
		createPublicQueryTransaction,
		createConfidentialQueryTransaction,
		createConfidentialTCertHQueryTransaction,
		createConfidentialECertHQueryTransaction,
	}

	// Init crypto layer
	Init()

	// Clenaup folders
	removeFolders()
}

func initPKI() {
	obcca.LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = obcca.NewECA()
	tca = obcca.NewTCA(eca)
	tlsca = obcca.NewTLSCA(eca)
}

func startPKI() {
	var opts []grpc.ServerOption
	if viper.GetBool("peer.pki.tls.enabled") {
		// TLS configuration
		creds, err := credentials.NewServerTLSFromFile(
			filepath.Join(viper.GetString("server.rootpath"), "tlsca.cert"),
			filepath.Join(viper.GetString("server.rootpath"), "tlsca.priv"),
		)
		if err != nil {
			panic("Failed creating credentials for OBC-CA: " + err.Error())
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	fmt.Printf("open socket...\n")
	sockp, err := net.Listen("tcp", viper.GetString("server.port"))
	if err != nil {
		panic("Cannot open port: " + err.Error())
	}
	fmt.Printf("open socket...done\n")

	server = grpc.NewServer(opts...)

	eca.Start(server)
	tca.Start(server)
	tlsca.Start(server)

	fmt.Printf("start serving...\n")
	server.Serve(sockp)
}

func initClients() error {
	// Deployer
	deployerConf := utils.NodeConfiguration{Type: "client", Name: "user1"}
	if err := RegisterClient(deployerConf.Name, ksPwd, deployerConf.GetEnrollmentID(), deployerConf.GetEnrollmentPWD()); err != nil {
		return err
	}
	var err error
	deployer, err = InitClient(deployerConf.Name, ksPwd)
	if err != nil {
		return err
	}

	// Invoker
	invokerConf := utils.NodeConfiguration{Type: "client", Name: "user2"}
	if err := RegisterClient(invokerConf.Name, ksPwd, invokerConf.GetEnrollmentID(), invokerConf.GetEnrollmentPWD()); err != nil {
		return err
	}
	invoker, err = InitClient(invokerConf.Name, ksPwd)
	if err != nil {
		return err
	}

	return nil
}

func initPeers() error {
	// Register
	conf := utils.NodeConfiguration{Type: "peer", Name: "peer"}
	err := RegisterPeer(conf.Name, ksPwd, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Verify that a second call to Register fails
	err = RegisterPeer(conf.Name, ksPwd, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Init
	peer, err = InitPeer(conf.Name, ksPwd)
	if err != nil {
		return err
	}

	err = RegisterPeer(conf.Name, ksPwd, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	return err
}

func initValidators() error {
	// Register
	conf := utils.NodeConfiguration{Type: "validator", Name: "validator"}
	err := RegisterValidator(conf.Name, ksPwd, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Verify that a second call to Register fails
	err = RegisterValidator(conf.Name, ksPwd, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Init
	validator, err = InitValidator(conf.Name, ksPwd)
	if err != nil {
		return err
	}

	err = RegisterValidator(conf.Name, ksPwd, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	return err
}

func createConfidentialDeployTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cds := &obc.ChaincodeDeploymentSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
		EffectiveDate: nil,
		CodePackage:   nil,
	}

	otx, err := obc.NewChaincodeDeployTransaction(cds, uuid)
	if err != nil {
		return nil, nil, err
	}
	tx, err := deployer.NewChaincodeDeployTransaction(cds, uuid)
	return otx, tx, err
}

func createConfidentialExecuteTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		return nil, nil, err
	}
	tx, err := invoker.NewChaincodeExecute(cis, uuid)
	return otx, tx, err
}

func createConfidentialQueryTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		return nil, nil, err
	}
	tx, err := invoker.NewChaincodeQuery(cis, uuid)
	return otx, tx, err
}

func createConfidentialTCertHDeployTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cds := &obc.ChaincodeDeploymentSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
		EffectiveDate: nil,
		CodePackage:   nil,
	}

	otx, err := obc.NewChaincodeDeployTransaction(cds, uuid)
	if err != nil {
		return nil, nil, err
	}
	handler, err := deployer.GetTCertificateHandlerNext()
	if err != nil {
		return nil, nil, err
	}
	txHandler, err := handler.GetTransactionHandler()
	if err != nil {
		return nil, nil, err
	}
	tx, err := txHandler.NewChaincodeDeployTransaction(cds, uuid)

	// Check binding consistency
	binding, _ := txHandler.GetBinding()
	if !reflect.DeepEqual(binding, utils.Hash(append(handler.GetCertificate(), tx.Nonce...))) {
		t.Fatal("Binding is malformed!")
	}

	// Check confidentiality level
	if tx.ConfidentialityLevel != cds.ChaincodeSpec.ConfidentialityLevel {
		t.Fatal("Failed setting confidentiality level")
	}

	// Check metadata
	if !reflect.DeepEqual(cds.ChaincodeSpec.Metadata, tx.Metadata) {
		t.Fatal("Failed copying metadata")
	}

	return otx, tx, err
}

func createConfidentialTCertHExecuteTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		return nil, nil, err
	}
	handler, err := invoker.GetTCertificateHandlerNext()
	if err != nil {
		return nil, nil, err
	}
	txHandler, err := handler.GetTransactionHandler()
	if err != nil {
		return nil, nil, err
	}
	tx, err := txHandler.NewChaincodeExecute(cis, uuid)

	// Check binding consistency
	binding, _ := txHandler.GetBinding()
	if !reflect.DeepEqual(binding, utils.Hash(append(handler.GetCertificate(), tx.Nonce...))) {
		t.Fatal("Binding is malformed!")
	}

	// Check confidentiality level
	if tx.ConfidentialityLevel != cis.ChaincodeSpec.ConfidentialityLevel {
		t.Fatal("Failed setting confidentiality level")
	}

	// Check metadata
	if !reflect.DeepEqual(cis.ChaincodeSpec.Metadata, tx.Metadata) {
		t.Fatal("Failed copying metadata")
	}

	return otx, tx, err
}

func createConfidentialTCertHQueryTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		return nil, nil, err
	}
	handler, err := invoker.GetTCertificateHandlerNext()
	if err != nil {
		return nil, nil, err
	}
	txHandler, err := handler.GetTransactionHandler()
	if err != nil {
		return nil, nil, err
	}
	tx, err := txHandler.NewChaincodeQuery(cis, uuid)

	// Check binding consistency
	binding, _ := txHandler.GetBinding()
	if !reflect.DeepEqual(binding, utils.Hash(append(handler.GetCertificate(), tx.Nonce...))) {
		t.Fatal("Binding is malformed!")
	}

	// Check confidentiality level
	if tx.ConfidentialityLevel != cis.ChaincodeSpec.ConfidentialityLevel {
		t.Fatal("Failed setting confidentiality level")
	}

	// Check metadata
	if !reflect.DeepEqual(cis.ChaincodeSpec.Metadata, tx.Metadata) {
		t.Fatal("Failed copying metadata")
	}

	return otx, tx, err
}

func createConfidentialECertHDeployTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cds := &obc.ChaincodeDeploymentSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
		EffectiveDate: nil,
		CodePackage:   nil,
	}

	otx, err := obc.NewChaincodeDeployTransaction(cds, uuid)
	if err != nil {
		return nil, nil, err
	}
	handler, err := deployer.GetEnrollmentCertificateHandler()
	if err != nil {
		return nil, nil, err
	}
	txHandler, err := handler.GetTransactionHandler()
	if err != nil {
		return nil, nil, err
	}
	tx, err := txHandler.NewChaincodeDeployTransaction(cds, uuid)

	// Check binding consistency
	binding, _ := txHandler.GetBinding()
	if !reflect.DeepEqual(binding, utils.Hash(append(handler.GetCertificate(), tx.Nonce...))) {
		t.Fatal("Binding is malformed!")
	}

	// Check confidentiality level
	if tx.ConfidentialityLevel != cds.ChaincodeSpec.ConfidentialityLevel {
		t.Fatal("Failed setting confidentiality level")
	}

	// Check metadata
	if !reflect.DeepEqual(cds.ChaincodeSpec.Metadata, tx.Metadata) {
		t.Fatal("Failed copying metadata")
	}

	return otx, tx, err
}

func createConfidentialECertHExecuteTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		return nil, nil, err
	}
	handler, err := invoker.GetEnrollmentCertificateHandler()
	if err != nil {
		return nil, nil, err
	}
	txHandler, err := handler.GetTransactionHandler()
	if err != nil {
		return nil, nil, err
	}
	tx, err := txHandler.NewChaincodeExecute(cis, uuid)

	// Check binding consistency
	binding, _ := txHandler.GetBinding()
	if !reflect.DeepEqual(binding, utils.Hash(append(handler.GetCertificate(), tx.Nonce...))) {
		t.Fatal("Binding is malformed!")
	}

	// Check confidentiality level
	if tx.ConfidentialityLevel != cis.ChaincodeSpec.ConfidentialityLevel {
		t.Fatal("Failed setting confidentiality level")
	}

	// Check metadata
	if !reflect.DeepEqual(cis.ChaincodeSpec.Metadata, tx.Metadata) {
		t.Fatal("Failed copying metadata")
	}

	return otx, tx, err
}

func createConfidentialECertHQueryTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_CONFIDENTIAL,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		return nil, nil, err
	}
	handler, err := invoker.GetEnrollmentCertificateHandler()
	if err != nil {
		return nil, nil, err
	}
	txHandler, err := handler.GetTransactionHandler()
	if err != nil {
		return nil, nil, err
	}
	tx, err := txHandler.NewChaincodeQuery(cis, uuid)

	// Check binding consistency
	binding, _ := txHandler.GetBinding()
	if !reflect.DeepEqual(binding, utils.Hash(append(handler.GetCertificate(), tx.Nonce...))) {
		t.Fatal("Binding is malformed!")
	}

	// Check confidentiality level
	if tx.ConfidentialityLevel != cis.ChaincodeSpec.ConfidentialityLevel {
		t.Fatal("Failed setting confidentiality level")
	}

	// Check metadata
	if !reflect.DeepEqual(cis.ChaincodeSpec.Metadata, tx.Metadata) {
		t.Fatal("Failed copying metadata")
	}

	return otx, tx, err
}

func createPublicDeployTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cds := &obc.ChaincodeDeploymentSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_PUBLIC,
		},
		EffectiveDate: nil,
		CodePackage:   nil,
	}

	otx, err := obc.NewChaincodeDeployTransaction(cds, uuid)
	if err != nil {
		return nil, nil, err
	}
	tx, err := deployer.NewChaincodeDeployTransaction(cds, uuid)
	return otx, tx, err
}

func createPublicExecuteTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_PUBLIC,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		return nil, nil, err
	}
	tx, err := invoker.NewChaincodeExecute(cis, uuid)
	return otx, tx, err
}

func createPublicQueryTransaction(t *testing.T) (*obc.Transaction, *obc.Transaction, error) {
	uuid := util.GenerateUUID()

	cis := &obc.ChaincodeInvocationSpec{
		ChaincodeSpec: &obc.ChaincodeSpec{
			Type:                 obc.ChaincodeSpec_GOLANG,
			ChaincodeID:          &obc.ChaincodeID{Path: "Contract001"},
			CtorMsg:              nil,
			ConfidentialityLevel: obc.ConfidentialityLevel_PUBLIC,
		},
	}

	otx, err := obc.NewChaincodeExecute(cis, uuid, obc.Transaction_CHAINCODE_QUERY)
	if err != nil {
		return nil, nil, err
	}
	tx, err := invoker.NewChaincodeQuery(cis, uuid)
	return otx, tx, err
}

func isEqual(src, dst *obc.Transaction) error {
	if !reflect.DeepEqual(src.Payload, dst.Payload) {
		return fmt.Errorf("Different Payload [%s]!=[%s].", utils.EncodeBase64(src.Payload), utils.EncodeBase64(dst.Payload))
	}

	if !reflect.DeepEqual(src.ChaincodeID, dst.ChaincodeID) {
		return fmt.Errorf("Different ChaincodeID [%s]!=[%s].", utils.EncodeBase64(src.ChaincodeID), utils.EncodeBase64(dst.ChaincodeID))
	}

	if !reflect.DeepEqual(src.Metadata, dst.Metadata) {
		return fmt.Errorf("Different Metadata [%s]!=[%s].", utils.EncodeBase64(src.Metadata), utils.EncodeBase64(dst.Metadata))
	}

	return nil
}

func cleanup() {
	fmt.Println("Cleanup...")
	CloseAllClients()
	CloseAllPeers()
	CloseAllValidators()
	stopPKI()
	removeFolders()
	fmt.Println("Cleanup...done!")
}

func stopPKI() {
	eca.Close()
	tca.Close()
	tlsca.Close()

	server.Stop()
}

func removeFolders() {
	if err := os.RemoveAll(filepath.Join(os.TempDir(), "obc-crypto-tests")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", "obc-crypto-tests", err)
	}

}

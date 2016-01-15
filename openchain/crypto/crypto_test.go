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
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/obc-ca/obcca"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"testing"
	_ "time"
)

var (
	validator Peer

	peer Peer

	deployer Client
	invoker  Client

	server *grpc.Server
	eca    *obcca.ECA
	tca    *obcca.TCA
	tlsca  *obcca.TLSCA
)

func TestMain(m *testing.M) {
	setup()

	// Init PKI
	go initPKI()
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

	ret := m.Run()

	cleanup()

	os.Exit(ret)
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

func TestClientDeployTransaction(t *testing.T) {
	_, tx, err := createConfidentialDeployTransaction()

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

func TestClientDeployTransaction2(t *testing.T) {
}

func TestClientExecuteTransaction(t *testing.T) {
	_, tx, err := createConfidentialExecuteTransaction()

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

func TestClientMultiExecuteTransaction(t *testing.T) {
	for i := 0; i < 24; i++ {
		_, tx, err := createConfidentialExecuteTransaction()

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

func TestPeerConfidentialDeployTransaction(t *testing.T) {
	_, tx, err := createConfidentialDeployTransaction()
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

func TestPeerConfidentialExecuteTransaction(t *testing.T) {
	_, tx, err := createConfidentialExecuteTransaction()
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

func TestPeerConfidentialQueryTransaction(t *testing.T) {
	_, tx, err := createConfidentialQueryTransaction()
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

func TestPeerPublicDeployTransaction(t *testing.T) {
	tx, err := createPublicDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating public deploy transaction [%s].", err)
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

func TestPeerPublicExecuteTransaction(t *testing.T) {
	tx, err := createPublicExecuteTransaction()
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

func TestPeerPublicQueryTransaction(t *testing.T) {
	tx, err := createPublicQueryTransaction()
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

func TestPeerStateEncryptor(t *testing.T) {
	_, deployTx, err := createConfidentialDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction [%s].", err)
	}
	_, invokeTxOne, err := createConfidentialExecuteTransaction()
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

func TestValidatorConfidentialDeployTransaction(t *testing.T) {
	otx, tx, err := createConfidentialDeployTransaction()
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

	if reflect.DeepEqual(res, tx) {
		t.Fatalf("Src and Dest Transaction should be different after PreExecution")
	}
	if err := isEqual(otx, res); err != nil {
		t.Fatalf("Decrypted transaction differs from the original: [%s]", err)
	}
}

func TestValidatorConfidentialExecuteTransaction(t *testing.T) {
	otx, tx, err := createConfidentialExecuteTransaction()
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
	if reflect.DeepEqual(res, tx) {
		t.Fatalf("Src and Dest Transaction should be different after PreExecution")
	}
	if err := isEqual(otx, res); err != nil {
		t.Fatalf("Decrypted transaction differs from the original: [%s]", err)
	}
}

func TestValidatorConfidentialQueryTransaction(t *testing.T) {
	_, deployTx, err := createConfidentialDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction [%s].", err)
	}
	_, invokeTxOne, err := createConfidentialExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction [%s].", err)
	}
	_, invokeTxTwo, err := createConfidentialExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction [%s].", err)
	}
	otx, queryTx, err := createConfidentialQueryTransaction()
	if err != nil {
		t.Fatalf("Failed creating query transaction [%s].", err)
	}

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

func TestValidatorPublicDeployTransaction(t *testing.T) {
	tx, err := createPublicDeployTransaction()
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

	// TODO:
	//	if reflect.DeepEqual(res, tx) {
	//		t.Fatalf("Src and Dest Transaction should be different after PreExecution")
	//	}
}

func TestValidatorPublicExecuteTransaction(t *testing.T) {
	tx, err := createPublicExecuteTransaction()
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
	// TODO:
	//	if reflect.DeepEqual(res, tx) {
	//		t.Fatalf("Src and Dest Transaction should be different after PreExecution")
	//	}
}

func TestValidatorStateEncryptor(t *testing.T) {
	_, deployTx, err := createConfidentialDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction [%s]", err)
	}
	_, invokeTxOne, err := createConfidentialExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction [%s]", err)
	}
	_, invokeTxTwo, err := createConfidentialExecuteTransaction()
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

func setup() {
	viper.SetConfigName("crypto_test") // name of config file (without extension)
	viper.AddConfigPath(".")           // path to look for the config file in
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file [%s] \n", err))
	}
	var formatter = logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} [%{module}] %{shortfunc} [%{shortfile}] -> %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	logging.SetFormatter(formatter)

	removeFolders()
}

func initPKI() {
	obcca.LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = obcca.NewECA()
	tca = obcca.NewTCA(eca)
	tlsca = obcca.NewTLSCA(eca)

	sockp, err := net.Listen("tcp", viper.GetString("server.port"))
	if err != nil {
		panic("Cannot open port: " + err.Error())
	}

	server = grpc.NewServer()

	eca.Start(server)
	tca.Start(server)
	tlsca.Start(server)

	server.Serve(sockp)

}

func initClients() error {
	// Deployer
	deployerConf := utils.NodeConfiguration{Type: "client", Name: "user1"}
	if err := RegisterClient(deployerConf.Name, nil, deployerConf.GetEnrollmentID(), deployerConf.GetEnrollmentPWD()); err != nil {
		return err
	}
	var err error
	deployer, err = InitClient(deployerConf.Name, nil)
	if err != nil {
		return err
	}

	// Invoker
	invokerConf := utils.NodeConfiguration{Type: "client", Name: "user2"}
	if err := RegisterClient(invokerConf.Name, nil, invokerConf.GetEnrollmentID(), invokerConf.GetEnrollmentPWD()); err != nil {
		return err
	}
	invoker, err = InitClient(invokerConf.Name, nil)
	if err != nil {
		return err
	}

	return nil
}

func initPeers() error {
	// Register
	conf := utils.NodeConfiguration{Type: "peer", Name: "peer"}
	err := RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Verify that a second call to Register fails
	err = RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Init
	peer, err = InitPeer(conf.Name, nil)
	if err != nil {
		return err
	}

	err = RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	return err
}

func initValidators() error {
	// Register
	conf := utils.NodeConfiguration{Type: "validator", Name: "validator"}
	err := RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Verify that a second call to Register fails
	err = RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Init
	validator, err = InitValidator(conf.Name, nil)
	if err != nil {
		return err
	}

	err = RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	return err
}

func createConfidentialDeployTransaction() (*obc.Transaction, *obc.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, nil, err
	}

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

func createConfidentialExecuteTransaction() (*obc.Transaction, *obc.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, nil, err
	}

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

func createConfidentialQueryTransaction() (*obc.Transaction, *obc.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, nil, err
	}

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

func createPublicDeployTransaction() (*obc.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, err
	}

	tx, err := deployer.NewChaincodeDeployTransaction(
		&obc.ChaincodeDeploymentSpec{
			ChaincodeSpec: &obc.ChaincodeSpec{
				Type:        obc.ChaincodeSpec_GOLANG,
				ChaincodeID: &obc.ChaincodeID{Path: "Contract001"},
				CtorMsg:     nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		uuid,
	)
	return tx, err
}

func createPublicExecuteTransaction() (*obc.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, err
	}
	tx, err := invoker.NewChaincodeExecute(
		&obc.ChaincodeInvocationSpec{
			ChaincodeSpec: &obc.ChaincodeSpec{
				Type:        obc.ChaincodeSpec_GOLANG,
				ChaincodeID: &obc.ChaincodeID{Path: "Contract001"},
				CtorMsg:     nil,
			},
		},
		uuid,
	)
	return tx, err
}

func createPublicQueryTransaction() (*obc.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, err
	}
	tx, err := invoker.NewChaincodeQuery(
		&obc.ChaincodeInvocationSpec{
			ChaincodeSpec: &obc.ChaincodeSpec{
				Type:        obc.ChaincodeSpec_GOLANG,
				ChaincodeID: &obc.ChaincodeID{Path: "Contract001"},
				CtorMsg:     nil,
			},
		},
		uuid,
	)
	return tx, err
}

func isEqual(src, dst *obc.Transaction) error {
	if !reflect.DeepEqual(src.Payload, dst.Payload) {
		return fmt.Errorf("Different Payload [%s]!=[%s].", utils.EncodeBase64(src.Payload), utils.EncodeBase64(dst.Payload))
	}

	if !reflect.DeepEqual(src.ChaincodeID, dst.ChaincodeID) {
		return fmt.Errorf("Different ChaincodeID [%s]!=[%s].", utils.EncodeBase64(src.ChaincodeID), utils.EncodeBase64(dst.ChaincodeID))
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
	if err := os.RemoveAll(viper.GetString("peer.fileSystemPath")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", viper.GetString("peer.fileSystemPath"), err)
	}
	if err := os.RemoveAll(viper.GetString("server.rootpath")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", viper.GetString("eca.crypto.path"), err)
	}
}

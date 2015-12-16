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
	pb "github.com/openblockchain/obc-peer/protos"

	"bytes"
	"fmt"
	"github.com/openblockchain/obc-peer/obcca/obcca"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

var (
	validator Peer

	peer Peer

	deployer Client
	invoker  Client

	caAlreadyOn bool
	eca         *obcca.ECA
	tca         *obcca.TCA
	caWaitGroup sync.WaitGroup
)

func TestMain(m *testing.M) {
	setup()

	// Init PKI
	go initPKI()
	defer cleanup()

	// Init clients
	err := initClients()
	if err != nil {
		fmt.Printf("Failed initializing clients: %s\n", err)
		panic(fmt.Errorf("Failed initializing clients: %s", err))
	}

	// Init peer
	err = initPeers()
	if err != nil {
		fmt.Printf("Failed initializing peers: %s\n", err)
		panic(fmt.Errorf("Failed initializing peers: %s", err))
	}

	// Init validators
	err = initValidators()
	if err != nil {
		fmt.Printf("Failed initializing validators: %s\n", err)
		panic(fmt.Errorf("Failed initializing validators: %s", err))
	}

	ret := m.Run()

	cleanup()

	os.Exit(ret)
}

func TestClientDeployTransaction(t *testing.T) {
	tx, err := createDeployTransaction()

	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}

	if tx == nil {
		t.Fatalf("Result must be different from nil")
	}

	// Check transaction. For test purposes only
	err = deployer.(*clientImpl).checkTransaction(tx)
	if err != nil {
		t.Fatalf("Failed checking transaction: %s", err)
	}
}

func TestClientExecuteTransaction(t *testing.T) {
	tx, err := createExecuteTransaction()

	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}

	if tx == nil {
		t.Fatalf("Result must be different from nil")
	}

	// Check transaction. For test purposes only
	err = invoker.(*clientImpl).checkTransaction(tx)
	if err != nil {
		t.Fatalf("Failed checking transaction: %s", err)
	}
}

func TestClientMultiExecuteTransaction(t *testing.T) {
	for i := 0; i < 24; i++ {
		tx, err := createExecuteTransaction()

		if err != nil {
			t.Fatalf("Failed creating execute transaction: %s", err)
		}

		if tx == nil {
			t.Fatalf("Result must be different from nil")
		}

		// Check transaction. For test purposes only
		err = invoker.(*clientImpl).checkTransaction(tx)
		if err != nil {
			t.Fatalf("Failed checking transaction: %s", err)
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

func TestPeerDeployTransaction(t *testing.T) {
	tx, err := createDeployTransaction()
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed creating transaction: %s", err)
	}

	res, err := peer.TransactionPreValidation(tx)
	if err != nil {
		t.Fatalf("Error must be nil: %s", err)
	}
	if res == nil {
		t.Fatalf("Result must be diffrent from nil")
	}

	res, err = peer.TransactionPreExecution(tx)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented: %s", err)
	}
	if res != nil {
		t.Fatalf("Result must nil")
	}
}

func TestPeerExecuteTransaction(t *testing.T) {
	tx, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed creating transaction: %s", err)
	}

	res, err := peer.TransactionPreValidation(tx)
	if err != nil {
		t.Fatalf("Error must be nil: %s", err)
	}
	if res == nil {
		t.Fatalf("Result must be diffrent from nil")
	}

	res, err = peer.TransactionPreExecution(tx)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented: %s", err)
	}
	if res != nil {
		t.Fatalf("Result must nil")
	}
}

func TestPeerStateEncryptor(t *testing.T) {
	deployTx, err := createDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}
	invokeTxOne, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction: %s", err)
	}

	res, err := peer.GetStateEncryptor(deployTx, invokeTxOne)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented: %s", err)
	}
	if res != nil {
		t.Fatalf("Result must be nil")
	}
}

func TestPeerSignVerify(t *testing.T) {
	msg := []byte("Hello World!!!")
	signature, err := peer.Sign(msg)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented: %s", err)
	}
	if signature != nil {
		t.Fatalf("Result must be nil")
	}

	err = peer.Verify(validator.GetID(), signature, msg)
	if err != utils.ErrNotImplemented {
		t.Fatalf("Error must be ErrNotImplemented: %s", err)
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
	tx, err := createDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}

	res, err := validator.TransactionPreValidation(tx)
	if err != nil {
		t.Fatalf("Error must be nil: %s", err)
	}
	if res == nil {
		t.Fatalf("Result must be diffrent from nil")
	}

	res, err = validator.TransactionPreExecution(tx)
	if err != nil {
		t.Fatalf("Error must be nil: %s", err)
	}
	if res == nil {
		t.Fatalf("Result must be diffrent from nil")
	}
}

func TestValidatorExecuteTransaction(t *testing.T) {
	tx, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating execute transaction: %s", err)
	}

	res, err := validator.TransactionPreValidation(tx)
	if err != nil {
		t.Fatalf("Error must be nil: %s", err)
	}
	if res == nil {
		t.Fatalf("Result must be diffrent from nil")
	}

	res, err = validator.TransactionPreExecution(tx)
	if err != nil {
		t.Fatalf("Error must be nil: %s", err)
	}
	if res == nil {
		t.Fatalf("Result must be diffrent from nil")
	}
}

func TestValidatorStateEncryptor(t *testing.T) {
	deployTx, err := createDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}
	invokeTxOne, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction: %s", err)
	}
	invokeTxTwo, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction: %s", err)
	}

	// Transactions must be PreExecuted by the validators before getting the StateEncryptor
	if _, err:=validator.TransactionPreValidation(deployTx); err != nil {
		t.Fatalf("Failed pre-validating deploty transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(deployTx); err != nil {
		t.Fatalf("Failed pre-validating deploty transaction: %s", err)
	}
	if _, err:=validator.TransactionPreValidation(invokeTxOne); err != nil {
		t.Fatalf("Failed pre-validating exec1 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(invokeTxOne); err != nil {
		t.Fatalf("Failed pre-validating exec1 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreValidation(invokeTxTwo); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(invokeTxTwo); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction: %s", err)
	}


	seOne, err := validator.GetStateEncryptor(deployTx, invokeTxOne)
	if err != nil {
		t.Fatalf("Failed creating state encryptor: %s", err)
	}
	pt := []byte("Hello World")
	aCt, err := seOne.Encrypt(pt)
	if err != nil {
		t.Fatalf("Failed encrypting state: %s", err)
	}
	aPt, err := seOne.Decrypt(aCt)
	if err != nil {
		t.Fatalf("Failed decrypting state: %s", err)
	}
	if !bytes.Equal(pt, aPt) {
		t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
	}

	seTwo, err := validator.GetStateEncryptor(deployTx, invokeTxTwo)
	if err != nil {
		t.Fatalf("Failed creating state encryptor: %s", err)
	}
	aPt2, err := seTwo.Decrypt(aCt)
	if err != nil {
		t.Fatalf("Failed decrypting state: %s", err)
	}
	if !bytes.Equal(pt, aPt2) {
		t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
	}

}

func TestValidatorQueryTransaction(t *testing.T) {
	deployTx, err := createDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}
	invokeTxOne, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction: %s", err)
	}
	invokeTxTwo, err := createExecuteTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction: %s", err)
	}
	queryTx, err := createQueryTransaction()
	if err != nil {
		t.Fatalf("Failed creating query transaction: %s", err)
	}

	// Transactions must be PreExecuted by the validators before getting the StateEncryptor
	if _, err:=validator.TransactionPreValidation(deployTx); err != nil {
		t.Fatalf("Failed pre-validating deploty transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(deployTx); err != nil {
		t.Fatalf("Failed pre-validating deploty transaction: %s", err)
	}
	if _, err:=validator.TransactionPreValidation(invokeTxOne); err != nil {
		t.Fatalf("Failed pre-validating exec1 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(invokeTxOne); err != nil {
		t.Fatalf("Failed pre-validating exec1 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreValidation(invokeTxTwo); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(invokeTxTwo); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreValidation(queryTx); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction: %s", err)
	}
	if _, err:=validator.TransactionPreExecution(queryTx); err != nil {
		t.Fatalf("Failed pre-validating exec2 transaction: %s", err)
	}


	// First invokeTx
	seOne, err := validator.GetStateEncryptor(deployTx, invokeTxOne)
	if err != nil {
		t.Fatalf("Failed creating state encryptor: %s", err)
	}
	pt := []byte("Hello World")
	aCt, err := seOne.Encrypt(pt)
	if err != nil {
		t.Fatalf("Failed encrypting state: %s", err)
	}
	aPt, err := seOne.Decrypt(aCt)
	if err != nil {
		t.Fatalf("Failed decrypting state: %s", err)
	}
	if !bytes.Equal(pt, aPt) {
		t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
	}

	// Second invokeTx
	seTwo, err := validator.GetStateEncryptor(deployTx, invokeTxTwo)
	if err != nil {
		t.Fatalf("Failed creating state encryptor: %s", err)
	}
	aPt2, err := seTwo.Decrypt(aCt)
	if err != nil {
		t.Fatalf("Failed decrypting state: %s", err)
	}
	if !bytes.Equal(pt, aPt2) {
		t.Fatalf("Failed decrypting state [%s != %s]: %s", string(pt), string(aPt), err)
	}

	// queryTx
	seThree, err := validator.GetStateEncryptor(deployTx, queryTx)
	ctQ, err := seThree.Encrypt(aPt2)
	if err != nil {
		t.Fatalf("Failed encrypting query result: %s", err)
	}

	aPt3, err := invoker.DecryptQueryResult(queryTx, ctQ)
	if err != nil {
		t.Fatalf("Failed decrypting query result: %s", err)
	}
	if !bytes.Equal(aPt2, aPt3) {
		t.Fatalf("Failed decrypting query result [%s != %s]: %s", string(aPt2), string(aPt3), err)
	}
}


func TestValidatorSignVerify(t *testing.T) {
	msg := []byte("Hello World!!!")
	signature, err := validator.Sign(msg)
	if err != nil {
		t.Fatalf("TestSign: failed generating signature: %s", err)
	}

	err = validator.Verify(validator.GetID(), signature, msg)
	if err != nil {
		t.Fatalf("TestSign: failed validating signature: %s", err)
	}
}

func setup() {
	viper.SetConfigName("crypto_test") // name of config file (without extension)
	viper.AddConfigPath(".")           // path to look for the config file in
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	removeFolders()
}

func initPKI() {
	// Check if the CAs are already up
	if err := utils.IsTCPPortOpen(viper.GetString("ports.ecaP")); err != nil {
		caAlreadyOn = true
		fmt.Println("Someone already listening")
		return
	}
	caAlreadyOn = false

	obcca.LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = obcca.NewECA()
	defer eca.Close()
	eca.Start(&caWaitGroup)

	tca = obcca.NewTCA(eca)
	defer tca.Close()
	tca.Start(&caWaitGroup)

	caWaitGroup.Wait()
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

func createDeployTransaction() (*pb.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, err
	}

	tx, err := deployer.NewChaincodeDeployTransaction(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:        pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		uuid,
	)
	return tx, err
}

func createExecuteTransaction() (*pb.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, err
	}
	tx, err := invoker.NewChaincodeExecute(
		&pb.ChaincodeInvocationSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:        pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
		},
		uuid,
	)
	return tx, err
}

func createQueryTransaction() (*pb.Transaction, error) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		return nil, err
	}
	tx, err := invoker.NewChaincodeQuery(
		&pb.ChaincodeInvocationSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:        pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
		},
		uuid,
	)
	return tx, err
}

func cleanup() {
	CloseAllClients()
	CloseAllValidators()
	killCAs()

	fmt.Println("Prepare to cleanup...")
	//	time.Sleep(40 * time.Second)

	fmt.Println("Test...")
	if err := utils.IsTCPPortOpen(viper.GetString("ports.ecaP")); err != nil {
		fmt.Println("AAA Someone already listening")
	}
	removeFolders()
	fmt.Println("Cleanup...done!")
}

func killCAs() {
	if !caAlreadyOn {
		eca.Stop()
		eca.Close()

		tca.Stop()
		tca.Close()
	}
}

func removeFolders() {
	if err := os.RemoveAll(viper.GetString("peer.fileSystemPath")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("peer.fileSystemPath"), err)
	}
	if err := os.RemoveAll(viper.GetString("eca.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("eca.crypto.path"), err)
	}
}

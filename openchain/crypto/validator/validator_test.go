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

package validator

import (
	pb "github.com/openblockchain/obc-peer/protos"

	"fmt"
	"github.com/openblockchain/obc-peer/obcca/obcca"
	"github.com/openblockchain/obc-peer/openchain/crypto/client"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

var validator *Validator
var mockClient *client.Client

var eca *obcca.ECA
var tca *obcca.TCA

var caAlreadyOn bool

var caWaitGroup sync.WaitGroup

func TestMain(m *testing.M) {
	setupTestConfig()

	// Init ECA
	go initMockCAs()
	defer cleanup()

	// Init a mock Client
	err := initMockClient()
	if err != nil {
		panic(fmt.Errorf("Failed initializing mock client: %s", err))
	}

	// New Validator
	validator = new(Validator)

	// Register
	err = validator.Register(getValidatorEnrollmentData())
	if err != nil {
		panic(fmt.Errorf("Failed registerting to the ECA: %s", err))
	}

	// Init
	err = validator.Init()
	var ret int
	if err != nil {
		panic(fmt.Errorf("Failed initializing: err %s", err))
	} else {
		ret = m.Run()
	}

	cleanup()

	os.Exit(ret)
}

func TestID(t *testing.T) {
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

func TestDeployTransaction(t *testing.T) {
	tx, err := mockDeployTransaction()
	if err != nil {
		t.Fatalf("Failed creating deploy transaction: %s", err)
	}

	res, err := validator.TransactionPreValidation(tx)
	if res == nil {
		t.Fatalf("TransactionPreValidation: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}

	res, err = validator.TransactionPreExecution(tx)
	if res == nil {
		t.Fatalf("TransactionPreExecution: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreExecution: failed pre validing transaction: %s", err)
	}
}

func TestInvokeTransaction(t *testing.T) {
	tx, err := mockInvokeTransaction()
	if err != nil {
		t.Fatalf("Failed creating invoke transaction: %s", err)
	}

	res, err := validator.TransactionPreValidation(tx)
	if res == nil {
		t.Fatalf("TransactionPreValidation: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreValidation: failed pre validing transaction: %s", err)
	}

	res, err = validator.TransactionPreExecution(tx)
	if res == nil {
		t.Fatalf("TransactionPreExecution: result must be diffrent from nil")
	}
	if err != nil {
		t.Fatalf("TransactionPreExecution: failed pre validing transaction: %s", err)
	}

	t.Log("tx to String: %s", tx.String())
}

func TestSignVerify(t *testing.T) {
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

func setupTestConfig() {
	viper.SetConfigName("validator_test") // name of config file (without extension)
	viper.AddConfigPath(".")              // path to look for the config file in
	err := viper.ReadInConfig()           // Find and read the config file
	if err != nil {                       // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	removeFolders()
}

func initMockCAs() {
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

func initMockClient() error {
	mockClient = new(client.Client)

	// Register
	err := mockClient.Register(getClientEnrollmentData())
	if err != nil {
		return err
	}

	// Init client
	return mockClient.Init()
}

func mockDeployTransaction() (*pb.Transaction, error) {
	tx, err := mockClient.NewChaincodeDeployTransaction(
		&pb.ChaincodeDeploymentSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:        pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		"uuid",
	)
	return tx, err
}

func mockInvokeTransaction() (*pb.Transaction, error) {
	tx, err := mockClient.NewChaincodeInvokeTransaction(
		&pb.ChaincodeInvocationSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:        pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
		},
		"uuid",
	)

	return tx, err
}

func getValidatorEnrollmentData() (string, string) {
	id := viper.GetString("validator.crypto.enrollid")
	if id == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'validator.crypto.enrollid' is set"))
	}

	pw := viper.GetString("validator.crypto.enrollpw")
	if id == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'validator.crypto.enrollpw' is set"))
	}

	return id, pw
}

func getClientEnrollmentData() (string, string) {
	id := viper.GetString("client.crypto.enrollid")
	if id == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'client.crypto.enrollid' is set"))
	}

	pw := viper.GetString("client.crypto.enrollpw")
	if id == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'client.crypto.enrollpw' is set"))
	}

	return id, pw
}

func cleanup() {
	mockClient.Close()
	validator.Close()
	killCAs()

	fmt.Println("Prepare to cleanup...")
	time.Sleep(20 * time.Second)

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
	if err := os.RemoveAll(viper.GetString("eca.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("eca.crypto.path"), err)
	}
	if err := os.RemoveAll(viper.GetString("client.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("client.crypto.path"), err)
	}
	if err := os.RemoveAll(viper.GetString("validator.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("validator.crypto.path"), err)
	}
}

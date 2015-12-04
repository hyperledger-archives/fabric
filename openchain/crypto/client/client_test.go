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

package client

import (
	"fmt"
	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/openblockchain/obc-peer/obcca/obcca"
	"github.com/spf13/viper"
	"io/ioutil"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"os"
	"sync"
	"testing"
	"time"
	_ "time"
)

var client *Client

var eca *obcca.ECA
var tca *obcca.TCA

var caWaitGroup sync.WaitGroup

func TestMain(m *testing.M) {
	setupTestConfig()

	// Init ECA and register the user using the Admin interface
	go initMockCAs()
	defer cleanup()

	// New Client
	client = new(Client)

	// Register
	usr, pwd, err := getEnrollmentData()
	if err != nil {
		killCAs()
		panic(fmt.Errorf("Failed getting enrollment data from config: %s", err))
	}

	err = client.Register(usr, pwd)
	if err != nil {
		killCAs()
		panic(fmt.Errorf("Failed registerting: %s", err))
	}

	// Init client
	err = client.Init()

	var ret int
	if err != nil {
		killCAs()
		panic(fmt.Errorf("Failed initializing: err %s", err))
	} else {
		ret = m.Run()
	}

	err = client.Close()
	if err != nil {
		panic(fmt.Errorf("Client Security Module:TestMain: failed cleanup: err %s", err))
	}

	cleanup()

	os.Exit(ret)
}

func Test_NewChaincodeDeployTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed generating uuid: err %s", err)
	}
	tx, err := client.NewChaincodeDeployTransaction(
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

	if err != nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed creating NewChaincodeDeployTransaction: err %s", err)
	}

	if tx == nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed creating NewChaincodeDeployTransaction: result is nil")
	}

	err = client.checkTransaction(tx)
	if err != nil {
		t.Fatalf("Test_NewChaincodeDeployTransaction: failed checking transaction: err %s", err)
	}
}

func Test_NewChaincodeInvokeTransaction(t *testing.T) {
	uuid, err := util.GenerateUUID()
	if err != nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed generating uuid: err %s", err)
	}
	tx, err := client.NewChaincodeInvokeTransaction(
		&pb.ChaincodeInvocationSpec{
			ChaincodeSpec: &pb.ChaincodeSpec{
				Type:        pb.ChaincodeSpec_GOLANG,
				ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
				CtorMsg:     nil,
			},
		},
		uuid,
	)

	if err != nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed creating NewChaincodeInvokeTransaction: err %s", err)
	}

	if tx == nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed creating NewChaincodeInvokeTransaction: result is nil")
	}

	err = client.checkTransaction(tx)
	if err != nil {
		t.Fatalf("Test_NewChaincodeInvokeTransaction: failed checking transaction: err %s", err)
	}
}

func Test_MultipleNewChaincodeInvokeTransaction(t *testing.T) {
	for i := 0; i < 24; i++ {
		uuid, err := util.GenerateUUID()
		if err != nil {
			t.Fatalf("Test_MultipleNewChaincodeInvokeTransaction: failed generating uuid: err %s", err)
		}
		tx, err := client.NewChaincodeInvokeTransaction(
			&pb.ChaincodeInvocationSpec{
				ChaincodeSpec: &pb.ChaincodeSpec{
					Type:        pb.ChaincodeSpec_GOLANG,
					ChaincodeID: &pb.ChaincodeID{Url: "Contract001", Version: "0.0.1"},
					CtorMsg:     nil,
				},
			},
			uuid,
		)

		if err != nil {
			t.Fatalf("Test_MultipleNewChaincodeInvokeTransaction: failed creating NewChaincodeInvokeTransaction: err %s", err)
		}

		if tx == nil {
			t.Fatalf("Test_MultipleNewChaincodeInvokeTransaction: failed creating NewChaincodeInvokeTransaction: result is nil")
		}

		err = client.checkTransaction(tx)
		if err != nil {
			t.Fatalf("Test_MultipleNewChaincodeInvokeTransaction: failed checking transaction: err %s", err)
		}

	}
}

func setupTestConfig() {
	viper.SetConfigName("client_test") // name of config file (without extension)
	viper.AddConfigPath(".")           // path to look for the config file in
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	removeFolders()
}

func initMockCAs() {
	// Check if the CAs are already up
	if err := utils.IsTCPPortOpen(viper.GetString("ports.ecaP")); err != nil {
		return
	}

	obcca.LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = obcca.NewECA()
	defer eca.Close()
	eca.Start(&caWaitGroup)

	tca = obcca.NewTCA(eca)
	defer tca.Close()
	tca.Start(&caWaitGroup)

	caWaitGroup.Wait()
}

func getEnrollmentData() (string, string, error) {
	id := viper.GetString("client.crypto.enrollid")
	if id == "" {
		return "", "", fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'client.crypto.enrollid' is set")
	}
	pw := viper.GetString("client.crypto.enrollpw")
	if pw == "" {
		return "", "", fmt.Errorf("Enrollment pw not specified in configuration file. Please check that property 'client.crypto.enrollpw' is set")
	}

	return id, pw, nil
}

func cleanup() {
	killCAs()
	client.Close()

	fmt.Println("Prepare to cleanup...")
	time.Sleep(10 * time.Second)

	fmt.Println("Test...")
	if err := utils.IsTCPPortOpen(viper.GetString("ports.ecaP")); err != nil {
		fmt.Println("AAA Someone already listening")
	}
	removeFolders()
	fmt.Println("Cleanup...done!")
}

func killCAs() {
	fmt.Println("Stopping CAs...")

	eca.Stop()
	tca.Stop()

	fmt.Println("Stopping CAs...done")
}

func removeFolders() {
	if err := os.RemoveAll(viper.GetString("eca.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("eca.crypto.path"), err)
	}
	if err := os.RemoveAll(viper.GetString("client.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s]: %s\n", viper.GetString("client.crypto.path"), err)
	}
}

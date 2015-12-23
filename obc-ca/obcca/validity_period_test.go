package obcca

import (
	pb "github.com/openblockchain/obc-peer/protos"

	//"bytes"
	"fmt"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/ledger/genesis"
	
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"errors"
	//"reflect"
)

var (
	validator crypto.Peer

	peer crypto.Peer

	deployer crypto.Client
	invoker  crypto.Client

	caAlreadyOn bool
	eca         *ECA
	tca         *TCA
	tlsca		*TLSCA
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
		fmt.Printf("Failed initializing clients [%s]\n", err.Error())
		panic(fmt.Errorf("Failed initializing clients [%s].", err.Error()))
	}

	// Init peer
	err = initPeers()
	if err != nil {
		fmt.Printf("Failed initializing peers [%s]\n", err.Error())
		panic(fmt.Errorf("Failed initializing peers [%s].", err.Error()))
	}

	// Init validators
	err = initValidators()
	if err != nil {
		fmt.Printf("Failed initializing validators [%s]\n", err.Error())
		panic(fmt.Errorf("Failed initializing validators [%s].", err.Error()))
	}
	
	viper.Set("pki.validity-period.update", "true")
	viper.Set("validator.validity-period.verification", "true")
	viper.Set("ledger.blockchain.deploy-system-chaincode","true")

	err = initBlockchain()
	
	if err != nil {
		fmt.Printf("Failed initializing blockchain [%s]\n", err.Error())
		panic(fmt.Errorf("Failed initializing blockchain [%s].", err.Error()))
	}

	
	ret := m.Run()

	cleanup()

	os.Exit(ret)
}



func initBlockchain() error {
	_, err := ledger.GetLedger()
	
	if err != nil {
		return err
	}
	
	
	
	err = genesis.MakeGenesis(nil)
	
	if err != nil {
		return err
	}
	
	return nil
	
}


func initValidators() error {
	// Register
	conf := utils.NodeConfiguration{Type: "validator", Name: "validator"}
	err := crypto.RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Verify that a second call to Register fails
	err = crypto.RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err == nil {
		return errors.New("Second call to Register didn't fail")
	}

	// Init
	validator, err = crypto.InitValidator(conf.Name, nil)
	if err != nil {
		return err
	}

	err = crypto.RegisterValidator(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	
	if err == nil {
		return errors.New("Third call to Register didn't fail")
	}

	return nil
}


func initPeers() error {
	// Register
	conf := utils.NodeConfiguration{Type: "peer", Name: "peer"}
	err := crypto.RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Verify that a second call to Register fails
	err = crypto.RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	// Init
	peer, err = crypto.InitPeer(conf.Name, nil)
	if err != nil {
		return err
	}

	err = crypto.RegisterPeer(conf.Name, nil, conf.GetEnrollmentID(), conf.GetEnrollmentPWD())
	if err != nil {
		return err
	}

	return err
}

func initClients() error {
	// Deployer
	deployerConf := utils.NodeConfiguration{Type: "client", Name: "user1"}
	if err := crypto.RegisterClient(deployerConf.Name, nil, deployerConf.GetEnrollmentID(), deployerConf.GetEnrollmentPWD()); err != nil {
		return err
	}
	var err error
	deployer, err = crypto.InitClient(deployerConf.Name, nil)
	if err != nil {
		return err
	}

	// Invoker
	invokerConf := utils.NodeConfiguration{Type: "client", Name: "user2"}
	if err := crypto.RegisterClient(invokerConf.Name, nil, invokerConf.GetEnrollmentID(), invokerConf.GetEnrollmentPWD()); err != nil {
		return err
	}
	invoker, err = crypto.InitClient(invokerConf.Name, nil)
	if err != nil {
		return err
	}

	return nil
}


func cleanup() {
	fmt.Println("Cleanup...")
	crypto.CloseAllClients()
	crypto.CloseAllPeers()
	crypto.CloseAllValidators()
	killCAs()
	removeFolders()
	fmt.Println("Cleanup...done!")
}

func killCAs() {
	if !caAlreadyOn {
		eca.Stop()
		eca.Close()

		tca.Stop()
		tca.Close()
		
		tlsca.Stop()
		tlsca.Close()
	}
}

func initPKI() {
	// Check if the CAs are already up
	if err := utils.IsTCPPortOpen(viper.GetString("ports.ecaP")); err != nil {
		caAlreadyOn = true
		fmt.Println("Someone already listening")
		return
	}
	caAlreadyOn = false

	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca = NewECA()
	defer eca.Close()
	eca.Start(&caWaitGroup)

	tca = NewTCA(eca)
	defer tca.Close()
	tca.Start(&caWaitGroup)

	tlsca = NewTLSCA()
	defer tlsca.Close()
	tlsca.Start(&caWaitGroup)

	caWaitGroup.Wait()
}

func setup() {
	viper.SetConfigName("validity_period_test") // name of config file (without extension)
	viper.AddConfigPath(".")           // path to look for the config file in
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file [%s] \n", err.Error()))
	}
	removeFolders()
}

func removeFolders() {
	if err := os.RemoveAll(viper.GetString("peer.fileSystemPath")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", viper.GetString("peer.fileSystemPath"), err)
	}
	if err := os.RemoveAll(viper.GetString("eca.crypto.path")); err != nil {
		fmt.Printf("Failed removing [%s] [%s]\n", viper.GetString("eca.crypto.path"), err)
	}
}

func TestClientDeployTransaction(t *testing.T) {
	tx, err := createConfidentialDeployTransaction()

	if err != nil {
		t.Fatalf("Failed creating deploy transaction [%s].", err.Error())
	}

	if tx == nil {
		t.Fatalf("Result must be different from nil")
	}

	// Check transaction. For test purposes only
//	err = deployer.(*clientImpl).checkTransaction(tx)
	//if err != nil {
		//t.Fatalf("Failed checking transaction [%s].", err.Error())
	//}
}

func createConfidentialDeployTransaction() (*pb.Transaction, error) {
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
				ConfidentialityLevel: pb.ConfidentialityLevel_CONFIDENTIAL,
			},
			EffectiveDate: nil,
			CodePackage:   nil,
		},
		uuid,
	)
	return tx, err
}
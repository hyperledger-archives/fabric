package client

import (
	pb "github.com/openblockchain/obc-peer/protos"

	"errors"
	"fmt"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/peer"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
)

var (
	logger = logging.MustGetLogger("main")

	id = "lukas"
	pw = "NPKYL39uKbkj"

	chaincodePath = "github.com/openblockchain/obc-peer/openchain/example/chaincode/asset_management/chaincode"
	chaincodeName = "assetmgm"
)

func main() {
	// Conf
	runtime.GOMAXPROCS(2)
	viper.SetEnvPrefix("openchain")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	// Login
	err := login()
	if err != nil {
		panic("Failed logging in")
	}

	// Deploy
	err = deploy()
	if err != nil {
		panic("Failed deploying")
	}
}

// getCliFilePath is a helper function to retrieve the local storage directory
// of client login tokens.
func getCliFilePath() string {
	localStore := viper.GetString("peer.fileSystemPath")
	if !strings.HasSuffix(localStore, "/") {
		localStore = localStore + "/"
	}
	localStore = localStore + "client/"
	return localStore
}

func login() (err error) {
	// Retrieve the CLI data storage path
	// Returns /var/openchain/production/client/
	localStore := getCliFilePath()
	logger.Info("Local data store for client loginToken: %s", localStore)

	// If the user is already logged in, return
	if _, err = os.Stat(localStore + "loginToken_" + id); err == nil {
		logger.Info("User '%s' is already logged in.\n", id)
		return
	}

	// Log in the user
	logger.Info("Logging in user '%s' on CLI interface...\n", id)

	// Get a devopsClient to perform the login
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		err = fmt.Errorf("Error trying to connect to local peer: %s", err)
		return
	}
	devopsClient := pb.NewDevopsClient(clientConn)

	// Build the login spec and login
	loginSpec := &pb.Secret{EnrollId: id, EnrollSecret: string(pw)}
	loginResult, err := devopsClient.Login(context.Background(), loginSpec)

	// Check if login is successful
	if loginResult.Status == pb.Response_SUCCESS {
		// If /var/openchain/production/client/ directory does not exist, create it
		if _, err := os.Stat(localStore); err != nil {
			if os.IsNotExist(err) {
				// Directory does not exist, create it
				if err := os.Mkdir(localStore, 0755); err != nil {
					panic(fmt.Errorf("Fatal error when creating %s directory: %s\n", localStore, err))
				}
			} else {
				// Unexpected error
				panic(fmt.Errorf("Fatal error on os.Stat of %s directory: %s\n", localStore, err))
			}
		}

		// Store client security context into a file
		logger.Info("Storing login token for user '%s'.\n", id)
		err = ioutil.WriteFile(localStore+"loginToken_"+id, []byte(id), 0755)
		if err != nil {
			panic(fmt.Errorf("Fatal error when storing client login token: %s\n", err))
		}

		logger.Info("Login successful for user '%s'.\n", id)
	} else {
		err = fmt.Errorf("Error on client login: %s", string(loginResult.Msg))
		return
	}

	return nil
}

func getDevopsClient() (pb.DevopsClient, error) {
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		return nil, fmt.Errorf("Error trying to connect to local peer: %s", err)
	}
	devopsClient := pb.NewDevopsClient(clientConn)
	return devopsClient, nil
}

// chaincodeDeploy deploys the chaincode. On success, the chaincode name
// (hash) is printed to STDOUT for use by subsequent chaincode-related CLI
// commands.
func deploy() (err error) {
	devopsClient, err := getDevopsClient()
	if err != nil {
		err = fmt.Errorf("Error building %s: %s", "init", err)
		return
	}

	// Build the spec

	input := &pb.ChaincodeInput{
		Function: "init",
	}
	spec := &pb.ChaincodeSpec{
		Type:        pb.ChaincodeSpec_GOLANG,
		ChaincodeID: &pb.ChaincodeID{Path: chaincodePath, Name: chaincodeName},
		CtorMsg:     input,
	}

	logger.Debug("Security is enabled. Include security context in deploy spec")
	if len(id) == 0 {
		err = errors.New("Must supply username for chaincode when security is enabled")
		return
	}

	// Retrieve the CLI data storage path
	// Returns /var/openchain/production/client/
	localStore := getCliFilePath()

	// Check if the user is logged in before sending transaction
	if _, err = os.Stat(localStore + "loginToken_" + id); err == nil {
		logger.Info("Local user '%s' is already logged in. Retrieving login token.\n", id)

		// Read in the login token
		token, err := ioutil.ReadFile(localStore + "loginToken_" + id)
		if err != nil {
			panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
		}

		// Add the login token to the chaincodeSpec
		spec.SecureContext = string(token)

		// If privacy is enabled, mark chaincode as confidential
		if viper.GetBool("security.privacy") {
			logger.Info("Set confidentiality level to CONFIDENTIAL.\n")
			spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
		}
	} else {
		// Check if the token is not there and fail
		if os.IsNotExist(err) {
			err = fmt.Errorf("User '%s' not logged in. Use the 'login' command to obtain a security token.", id)
			return
		}
		// Unexpected error
		panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
	}

	chaincodeDeploymentSpec, err := devopsClient.Deploy(context.Background(), spec)
	if err != nil {
		err = fmt.Errorf("Error building %s: %s\n", "init", err)
		return
	}
	logger.Info("Deploy result: %s", chaincodeDeploymentSpec.ChaincodeSpec)
	fmt.Println(chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name)
	return nil
}

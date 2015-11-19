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

package chaincode

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	"github.com/openblockchain/obc-peer/openchain/container"
	pb "github.com/openblockchain/obc-peer/protos"
)

var chaincodeLog = logging.MustGetLogger("chaincode")

// ChainName is the name of the chain to which this chaincode support belongs to.
type ChainName string

const (
	// DefaultChain is the name of the default chain.
	DefaultChain ChainName = "default"
	// DevModeUserRunsChaincode property allows user to run chaincode in development environment
	DevModeUserRunsChaincode    string = "dev_mode"
	chaincodeInstallPathDefault string = "/go/bin/"
	peerAddressDefault          string = "0.0.0.0:30303"
)

// chains is a map between different blockchains and their ChaincodeSupport.
//this needs to be a first class, top-level object... for now, lets just have a placeholder
var chains map[ChainName]*ChaincodeSupport

// ccStartupTimeout is the timeout after which deploy will fail.
var ccStartupTimeout time.Duration
var chaincodeInstallPath string

// UserRunsCC is true when user is running the chaincode in dev mode and no container is launched.
var UserRunsCC bool

var peerAddress string

func init() {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigName("openchain") // name of config file (without extension)
	viper.AddConfigPath("./")        // path to look for the config file in
	viper.AddConfigPath("./../")     // path to look for the config file in
	viper.AddConfigPath("./../../")  // path to look for the config file in
	err := viper.ReadInConfig()      // Find and read the config file
	if err != nil {
		fmt.Printf("Error reading viper :%s\n", err)
	}

	chains = make(map[ChainName]*ChaincodeSupport)
	to, err := strconv.Atoi(viper.GetString("chaincode.startuptimeout"))
	if err != nil { //what went wrong ?
		fmt.Printf("could not retrive timeout var...setting to 5secs\n")
		to = 5000
	}
	ccStartupTimeout = time.Duration(to) * time.Millisecond

	mode := viper.GetString("chaincode.chaincoderunmode")
	if mode == DevModeUserRunsChaincode {
		UserRunsCC = true
	} else {
		UserRunsCC = false
	}

	chaincodeInstallPath = viper.GetString("chaincode.chaincodeinstallpath")
	if chaincodeInstallPath == "" {
		chaincodeInstallPath = chaincodeInstallPathDefault
	}

	peerAddress = viper.GetString("peer.address")
	if peerAddress == "" {
		peerAddress = peerAddressDefault
	}

	fmt.Printf("chaincode env using [startuptimeout-%d, chaincode run mode-%s, peer address-%s]\n", to, mode, peerAddress)
}

// handlerMap maps chaincodeIDs to their handlers, and maps Uuids to bool
type handlerMap struct {
	sync.RWMutex
	// Handlers for each chaincode
	chaincodeMap map[string]*Handler
}

// GetChain returns the name of the chain to which this chaincode support belongs
func GetChain(name ChainName) *ChaincodeSupport {
	return chains[name]
}

// NewChaincodeSupport creates a new ChaincodeSupport instance
func NewChaincodeSupport() *ChaincodeSupport {
	//we need to pass chainname when we do multiple chains...till then use DefaultChain
	s := &ChaincodeSupport{name: DefaultChain, handlerMap: &handlerMap{chaincodeMap: make(map[string]*Handler)}}

	//initialize global chain
	chains[DefaultChain] = s

	return s
}

// // ChaincodeStream standard stream for ChaincodeMessage type.
// type ChaincodeStream interface {
// 	Send(*pb.ChaincodeMessage) error
// 	Recv() (*pb.ChaincodeMessage, error)
// }

// ChaincodeSupport responsible for providing interfacing with chaincodes from the Peer.
type ChaincodeSupport struct {
	name       ChainName
	handlerMap *handlerMap
}

// DuplicateChaincodeHandlerError returned if attempt to register same chaincodeID while a stream already exists.
type DuplicateChaincodeHandlerError struct {
	ChaincodeID *pb.ChaincodeID
}

func (d *DuplicateChaincodeHandlerError) Error() string {
	return fmt.Sprintf("Duplicate chaincodeID error: %s", d.ChaincodeID)
}

func newDuplicateChaincodeHandlerError(chaincodeHandler *Handler) error {
	return &DuplicateChaincodeHandlerError{ChaincodeID: chaincodeHandler.ChaincodeID}
}

// getChaincodeID constructs the ID from pb.ChaincodeID; used by handlerMap
func getChaincodeID(cID *pb.ChaincodeID) (string, error) {
	if cID == nil {
		return "", fmt.Errorf("Cannot construct chaincodeID, got nil object")
	}
	return cID.Url + ":" + cID.Version, nil
}

func (chaincodeSupport *ChaincodeSupport) registerHandler(chaincodehandler *Handler) error {
	key, err := getChaincodeID(chaincodehandler.ChaincodeID)

	if err != nil {
		return fmt.Errorf("Error registering handler: %s", err)
	}
	chaincodeSupport.handlerMap.Lock()
	defer chaincodeSupport.handlerMap.Unlock()

	h2, ok := chaincodeSupport.handlerMap.chaincodeMap[key]
	if ok && h2.registered == true {
		chaincodeLogger.Debug("duplicate registered handler(key:%s) return error", key)
		// Duplicate, return error
		return newDuplicateChaincodeHandlerError(chaincodehandler)
	}
	//a placeholder, unregistered handler will be setup by query or transaction processing that comes
	//through via consensus. In this case we swap the handler and give it the notify channel
	if h2 != nil {
		chaincodehandler.readyNotify = h2.readyNotify
		delete(chaincodeSupport.handlerMap.chaincodeMap, key)
	}

	chaincodeSupport.handlerMap.chaincodeMap[key] = chaincodehandler

	chaincodehandler.registered = true

	//now we are ready to receive messages and send back responses
	chaincodehandler.responseNotifiers = make(map[string]chan *pb.ChaincodeMessage)
	chaincodehandler.uuidMap = make(map[string]bool)

	chaincodeLogger.Debug("registered handler complete for chaincode %s", key)

	return nil
}

func (chaincodeSupport *ChaincodeSupport) deregisterHandler(chaincodehandler *Handler) error {
	key, err := getChaincodeID(chaincodehandler.ChaincodeID)
	if err != nil {
		return fmt.Errorf("Error deregistering handler: %s", err)
	}
	chaincodeSupport.handlerMap.Lock()
	defer chaincodeSupport.handlerMap.Unlock()
	if _, ok := chaincodeSupport.handlerMap.chaincodeMap[key]; !ok {
		// Handler NOT found
		return fmt.Errorf("Error deregistering handler, could not find handler with key: %s", key)
	}
	delete(chaincodeSupport.handlerMap.chaincodeMap, key)
	chaincodeLogger.Debug("Deregistered handler with key: %s", key)
	return nil
}

// GetExecutionContext returns the execution context.  DEPRECATED. TO be removed.
func (chaincodeSupport *ChaincodeSupport) GetExecutionContext(context context.Context, requestContext *pb.ChaincodeRequestContext) (*pb.ChaincodeExecutionContext, error) {
	//chaincodeId := &pb.ChaincodeIdentifier{Url: "github."}
	timeStamp := &google_protobuf.Timestamp{Seconds: time.Now().UnixNano(), Nanos: 0}
	executionContext := &pb.ChaincodeExecutionContext{ChaincodeId: requestContext.GetId(),
		Timestamp: timeStamp}

	chaincodeLog.Debug("returning execution context: %s", executionContext)
	return executionContext, nil
}

// Based on state of chaincode send either init or ready to move to ready state
func (chaincodeSupport *ChaincodeSupport) sendInitOrReady(context context.Context, uuid string, chaincode string, f *string, initArgs []string, timeout time.Duration) error {
	chaincodeSupport.handlerMap.Lock()
	//if its in the map, there must be a connected stream...nothing to do
	var handler *Handler
	var ok bool
	if handler, ok = chaincodeSupport.handlerMap.chaincodeMap[chaincode]; !ok {
		chaincodeSupport.handlerMap.Unlock()
		chaincodeLog.Debug("[sendInitOrRead]handler not found for chaincode %s", chaincode)
		return fmt.Errorf("handler not found for chaincode %s", chaincode)
	}
	chaincodeSupport.handlerMap.Unlock()

	var notfy chan *pb.ChaincodeMessage
	var err error
	if notfy, err = handler.initOrReady(uuid, f, initArgs); err != nil {
		return fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)
	}
	if notfy != nil {
		select {
		case ccMsg := <-notfy:
			if ccMsg.Type == pb.ChaincodeMessage_ERROR {
				return fmt.Errorf("Error initializing container %s: %s", chaincode, string(ccMsg.Payload))
			}
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("Timeout expired while executing send init message")
		}
	}
	return err
}

// launchAndWaitForRegister will launch container if not already running
func (chaincodeSupport *ChaincodeSupport) launchAndWaitForRegister(context context.Context, cID *pb.ChaincodeID, uuid string) (bool, error) {
	vmname, err := container.GetVMName(cID)
	if err != nil {
		return false, fmt.Errorf("[launchAndWaitForRegister]Error getting vm name: %s", err)
	}
	chaincode, err := getChaincodeID(cID)
	if err != nil {
		return false, fmt.Errorf("[launchAndWaitForRegister]Error getting chaincode: %s", err)
	}

	chaincodeSupport.handlerMap.Lock()
	//if its in the map, there must be a connected stream...nothing to do
	if _, ok := chaincodeSupport.handlerMap.chaincodeMap[chaincode]; ok {
		chaincodeLog.Debug("[launchAndWaitForRegister] chaincode is running and ready: %s", chaincode)
		chaincodeSupport.handlerMap.Unlock()
		return true, nil
	}
	alreadyRunning := false
	//register placeholder Handler. This will be transferred in registerHandler
	notfy := make(chan bool, 1)
	chaincodeSupport.handlerMap.chaincodeMap[chaincode] = &Handler{readyNotify: notfy}

	chaincodeSupport.handlerMap.Unlock()

	//launch the chaincode
	//creat a StartImageReq obj and send it to VMCProcess
	sir := container.StartImageReq{ID: vmname, Detach: true}
	_, err = container.VMCProcess(context, "Docker", sir)
	if err != nil {
		err = fmt.Errorf("Error starting container: %s", err)
		chaincodeSupport.handlerMap.Lock()
		delete(chaincodeSupport.handlerMap.chaincodeMap, chaincode)
		chaincodeSupport.handlerMap.Unlock()
		return alreadyRunning, err
	}

	//wait for REGISTER state
	select {
	case ok := <-notfy:
		if !ok {
			err = fmt.Errorf("registration failed for %s(tx:%s)", vmname, uuid)
		}
	case <-time.After(ccStartupTimeout):
		err = fmt.Errorf("Timeout expired while starting chaincode %s(tx:%s)", vmname, uuid)
		chaincodeSupport.handlerMap.Lock()
		delete(chaincodeSupport.handlerMap.chaincodeMap, chaincode)
		chaincodeSupport.handlerMap.Unlock()
	}
	if err != nil {
		//TODO stop the container
	}
	return alreadyRunning, err
}

func (chaincodeSupport *ChaincodeSupport) stopChaincode(context context.Context, cID *pb.ChaincodeID) error {
	vmname, err := container.GetVMName(cID)
	if err != nil {
		return fmt.Errorf("[stopChaincode]Error getting vm name: %s", err)
	}

	chaincode, err := getChaincodeID(cID)
	if err != nil {
		return fmt.Errorf("[stopChaincode]Error getting chaincode: %s", err)
	}

	//stop the chaincode
	sir := container.StopImageReq{ID: vmname, Timeout: 0}

	_, err = container.VMCProcess(context, "Docker", sir)
	if err != nil {
		err = fmt.Errorf("Error stopping container: %s", err)
		//but proceed to cleanup
	}

	chaincodeSupport.handlerMap.Lock()
	if _, ok := chaincodeSupport.handlerMap.chaincodeMap[chaincode]; !ok {
		//nothing to do
		chaincodeSupport.handlerMap.Unlock()
		return nil
	}

	delete(chaincodeSupport.handlerMap.chaincodeMap, chaincode)

	chaincodeSupport.handlerMap.Unlock()

	return err
}

// LaunchChaincode will launch the chaincode if not running (if running return nil) and will wait for handler of the chaincode to get into FSM ready state.
func (chaincodeSupport *ChaincodeSupport) LaunchChaincode(context context.Context, t *pb.Transaction) (*pb.ChaincodeID, *pb.ChaincodeInput, error) {
	//build the chaincode
	var cID *pb.ChaincodeID
	var cMsg *pb.ChaincodeInput
	var f *string
	var initargs []string
	if t.Type == pb.Transaction_CHAINCODE_NEW {
		cds := &pb.ChaincodeDeploymentSpec{}
		err := proto.Unmarshal(t.Payload, cds)
		if err != nil {
			return nil, nil, err
		}
		cID = cds.ChaincodeSpec.ChaincodeID
		cMsg = cds.ChaincodeSpec.CtorMsg
		f = &cMsg.Function
		initargs = cMsg.Args
	} else if t.Type == pb.Transaction_CHAINCODE_EXECUTE || t.Type == pb.Transaction_CHAINCODE_QUERY {
		ci := &pb.ChaincodeInvocationSpec{}
		err := proto.Unmarshal(t.Payload, ci)
		if err != nil {
			return nil, nil, err
		}
		cID = ci.ChaincodeSpec.ChaincodeID
		cMsg = ci.ChaincodeSpec.CtorMsg
	} else {
		chaincodeSupport.handlerMap.Unlock()
		return nil, nil, fmt.Errorf("invalid transaction type: %d", t.Type)
	}
	chaincode, err := getChaincodeID(cID)
	if err != nil {
		return cID, cMsg, err
	}

	chaincodeSupport.handlerMap.Lock()
	//if its in the map, there must be a connected stream...nothing to do
	if handler, ok := chaincodeSupport.handlerMap.chaincodeMap[chaincode]; ok {
		if handler.isRunning() {
			chaincodeLog.Debug("[LaunchChainCode] chaincode is running : %s", chaincode)
			chaincodeSupport.handlerMap.Unlock()
			return cID, cMsg, nil
		}
		chaincodeLog.Debug("Container not in READY state. It is in state %s", handler.FSM.Current())
	}
	chaincodeSupport.handlerMap.Unlock()

	//from here on : if we launch the container and get an error, we need to stop the container
	if !UserRunsCC {
		_, err = chaincodeSupport.launchAndWaitForRegister(context, cID, t.Uuid)
		if err != nil {
			chaincodeLog.Debug("launchAndWaitForRegister failed %s", err)
			return cID, cMsg, err
		}
	}

	if err == nil {
		//send init (if (f,args)) and wait for ready state
		err = chaincodeSupport.sendInitOrReady(context, t.Uuid, chaincode, f, initargs, ccStartupTimeout)
		if err != nil {
			chaincodeLog.Debug("sending init failed(%s)", err)
			err = fmt.Errorf("Failed to init chaincode(%s)", err)
			errIgnore := chaincodeSupport.stopChaincode(context, cID)
			if errIgnore != nil {
				chaincodeLog.Debug("stop failed %s(%s)", errIgnore, err)
			}
		}
		chaincodeLog.Debug("sending init completed")
	}

	chaincodeLog.Debug("LaunchChaincode complete\n")

	return cID, cMsg, err
}

// DeployChaincode deploys the chaincode if not in development mode where user is running the chaincode.
func (chaincodeSupport *ChaincodeSupport) DeployChaincode(context context.Context, t *pb.Transaction) (*pb.ChaincodeDeploymentSpec, error) {
	if UserRunsCC {
		chaincodeLog.Debug("command line, not deploying chaincode")
		return nil, nil
	}

	//build the chaincode
	cds := &pb.ChaincodeDeploymentSpec{}
	err := proto.Unmarshal(t.Payload, cds)
	if err != nil {
		return nil, err
	}
	cID := cds.ChaincodeSpec.ChaincodeID
	chaincode, err := getChaincodeID(cID)
	if err != nil {
		return cds, err
	}
	chaincodeSupport.handlerMap.Lock()
	//if its in the map, there must be a connected stream...and we are trying to build the code ?!
	if _, ok := chaincodeSupport.handlerMap.chaincodeMap[chaincode]; ok {
		chaincodeLog.Debug("deploy ?!! there's a chaincode with that name running: %s", chaincode)
		chaincodeSupport.handlerMap.Unlock()
		return cds, fmt.Errorf("deploy attempted but a chaincode with same name running %s", chaincode)
	}
	chaincodeSupport.handlerMap.Unlock()

	//create build request ...
	vmname, err := container.GetVMName(cds.ChaincodeSpec.ChaincodeID)

	//openchain.yaml in the container likely will not have the right url:version. We know the right
	//values, lets construct and pass as envs
	var targz io.Reader = bytes.NewBuffer(cds.CodePackage)
	envs := []string{"OPENCHAIN_CHAINCODE_ID_URL=" + cID.Url, "OPENCHAIN_CHAINCODE_ID_VERSION=" + cID.Version, "OPENCHAIN_PEER_ADDRESS=" + peerAddress}
	toks := strings.Split(vmname, ":")
	if toks == nil {
		return cds, fmt.Errorf("cannot get version from %s", vmname)
	}

	toks = strings.Split(toks[0], ".")
	if toks == nil {
		return cds, fmt.Errorf("cannot get path components from %s", vmname)
	}

	//TODO : chaincode executable will be same as the name of the last folder (golang thing...)
	//       need to revisit executable name assignment
	//e.g, for path github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example01
	//     exec is "chaincode_example01"
	exec := []string{chaincodeInstallPath + toks[len(toks)-1]}
	chaincodeLog.Debug("Executable is %s", exec[0])

	cir := &container.CreateImageReq{ID: vmname, Args: exec, Reader: targz, Env: envs}

	chaincodeLog.Debug("deploying vmname %s", vmname)
	//create image and create container
	_, err = container.VMCProcess(context, "Docker", cir)
	if err != nil {
		err = fmt.Errorf("Error starting container: %s", err)
	}

	return cds, err
}

// Register the bidi stream entry point called by chaincode to register with the Peer.
func (chaincodeSupport *ChaincodeSupport) Register(stream pb.ChaincodeSupport_RegisterServer) error {
	return HandleChaincodeStream(chaincodeSupport, stream)
}

// createTransactionMessage creates a transaction message.
func createTransactionMessage(uuid string, cMsg *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	payload, err := proto.Marshal(cMsg)
	if err != nil {
		fmt.Printf(err.Error())
		return nil, err
	}
	return &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_TRANSACTION, Payload: payload, Uuid: uuid}, nil
}

// createQueryMessage creates a query message.
func createQueryMessage(uuid string, cMsg *pb.ChaincodeInput) (*pb.ChaincodeMessage, error) {
	payload, err := proto.Marshal(cMsg)
	if err != nil {
		return nil, err
	}
	return &pb.ChaincodeMessage{Type: pb.ChaincodeMessage_QUERY, Payload: payload, Uuid: uuid}, nil
}

// Execute executes a transaction and waits for it to complete until a timeout value.
func (chaincodeSupport *ChaincodeSupport) Execute(ctxt context.Context, chaincode string, msg *pb.ChaincodeMessage, timeout time.Duration) (*pb.ChaincodeMessage, error) {
	chaincodeSupport.handlerMap.Lock()
	//we expect the chaincode to be running... sanity check
	handler, ok := chaincodeSupport.handlerMap.chaincodeMap[chaincode]
	if !ok {
		chaincodeSupport.handlerMap.Unlock()
		chaincodeLog.Debug("[Execute]chaincode is not running: %s", chaincode)
		return nil, fmt.Errorf("Cannot execute transaction or query for %s", chaincode)
	}
	chaincodeSupport.handlerMap.Unlock()

	var notfy chan *pb.ChaincodeMessage
	var err error
	if notfy, err = handler.sendExecuteMessage(msg); err != nil {
		return nil, fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)
	}
	select {
	case ccresp := <-notfy:
		//we delete the now that it has been delivered
		handler.deleteNotifier(msg.Uuid)
		return ccresp, nil
	case <-time.After(timeout):
		//we delete the now that we are going away (under lock, in case chaincode comes back JIT)
		handler.deleteNotifier(msg.Uuid)
		return nil, fmt.Errorf("Timeout expired while executing transaction")
	}
}

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
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	pb "github.com/openblockchain/obc-peer/protos"
)

var chaincodeLog = logging.MustGetLogger("chaincode")

// ChainName is the name of the chain to which this chaincode support belongs to.
type ChainName string

const (
	// DefaultChain is the name of the default chain.
	DefaultChain ChainName = "default"
	// DevModeUserRunsChaincode property allows user to run chaincode in development environment
	DevModeUserRunsChaincode       string = "dev"
	chaincodeStartupTimeoutDefault int    = 5000
	chaincodeInstallPathDefault    string = "/opt/gopath/bin/"
	peerAddressDefault             string = "0.0.0.0:30303"
)

// chains is a map between different blockchains and their ChaincodeSupport.
//this needs to be a first class, top-level object... for now, lets just have a placeholder
var chains map[ChainName]*ChaincodeSupport

func init() {
	chains = make(map[ChainName]*ChaincodeSupport)
}

// handlerMap maps chaincodeIDs to their handlers, and maps Uuids to bool
type handlerMap struct {
	sync.RWMutex
	// Handlers for each chaincode
	chaincodeMap map[string]*Handler
}

// GetChain returns the chaincode support for a given chain
func GetChain(name ChainName) *ChaincodeSupport {
	return chains[name]
}

//call this under lock
func (chaincodeSupport *ChaincodeSupport) preLaunchSetup(chaincode string) chan bool {
	//register placeholder Handler. This will be transferred in registerHandler
	//NOTE: from this point, existence of handler for this chaincode means the chaincode
	//is in the process of getting started (or has been started)
	notfy := make(chan bool, 1)
	chaincodeSupport.handlerMap.chaincodeMap[chaincode] = &Handler{readyNotify: notfy}
	return notfy
}

//call this under lock
func (chaincodeSupport *ChaincodeSupport) chaincodeHasBeenLaunched(chaincode string) (*Handler, bool) {
	handler, hasbeenlaunched := chaincodeSupport.handlerMap.chaincodeMap[chaincode]
	return handler, hasbeenlaunched
}

// NewChaincodeSupport creates a new ChaincodeSupport instance
func NewChaincodeSupport(chainname ChainName, getPeerEndpoint func() (*pb.PeerEndpoint, error), userrunsCC bool, ccstartuptimeout time.Duration, secHelper crypto.Peer) *ChaincodeSupport {
	s := &ChaincodeSupport{name: chainname, handlerMap: &handlerMap{chaincodeMap: make(map[string]*Handler)}, secHelper: secHelper}

	//initialize global chain
	chains[chainname] = s

	peerEndpoint, err := getPeerEndpoint()
	if err != nil {
		chaincodeLog.Error(fmt.Sprintf("Error getting PeerEndpoint, using peer.address: %s", err))
		s.peerAddress = viper.GetString("peer.address")
	} else {
		s.peerAddress = peerEndpoint.Address
	}
	chaincodeLog.Info("Chaincode support using peerAddress: %s\n", s.peerAddress)
	//peerAddress = viper.GetString("peer.address")
	if s.peerAddress == "" {
		s.peerAddress = peerAddressDefault
	}

	s.userRunsCC = userrunsCC

	s.ccStartupTimeout = ccstartuptimeout * time.Millisecond

	//TODO I'm not sure if this needs to be on a per chain basis... too lowel and just needs to be a global default ?
	s.chaincodeInstallPath = chaincodeInstallPathDefault

	return s
}

// // ChaincodeStream standard stream for ChaincodeMessage type.
// type ChaincodeStream interface {
// 	Send(*pb.ChaincodeMessage) error
// 	Recv() (*pb.ChaincodeMessage, error)
// }

// ChaincodeSupport responsible for providing interfacing with chaincodes from the Peer.
type ChaincodeSupport struct {
	name                 ChainName
	handlerMap           *handlerMap
	peerAddress          string
	ccStartupTimeout     time.Duration
	chaincodeInstallPath string
	userRunsCC           bool
	secHelper            crypto.Peer
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

func (chaincodeSupport *ChaincodeSupport) registerHandler(chaincodehandler *Handler) error {
	key := chaincodehandler.ChaincodeID.Name

	chaincodeSupport.handlerMap.Lock()
	defer chaincodeSupport.handlerMap.Unlock()

	h2, ok := chaincodeSupport.chaincodeHasBeenLaunched(key)
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
	chaincodehandler.txCtxs = make(map[string]*transactionContext)
	chaincodehandler.uuidMap = make(map[string]bool)
	chaincodehandler.isTransaction = make(map[string]bool)

	chaincodeLogger.Debug("registered handler complete for chaincode %s", key)

	return nil
}

func (chaincodeSupport *ChaincodeSupport) deregisterHandler(chaincodehandler *Handler) error {

	// clean up rangeQueryIteratorMap
	for _, context := range chaincodehandler.txCtxs {
		for _, v := range context.rangeQueryIteratorMap {
			v.Close()
		}
	}

	key := chaincodehandler.ChaincodeID.Name
	chaincodeLogger.Debug("Deregister handler: %s", key)
	chaincodeSupport.handlerMap.Lock()
	defer chaincodeSupport.handlerMap.Unlock()
	if _, ok := chaincodeSupport.chaincodeHasBeenLaunched(key); !ok {
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
func (chaincodeSupport *ChaincodeSupport) sendInitOrReady(context context.Context, uuid string, chaincode string, f *string, initArgs []string, timeout time.Duration, tx *pb.Transaction, depTx *pb.Transaction) error {
	chaincodeSupport.handlerMap.Lock()
	//if its in the map, there must be a connected stream...nothing to do
	var handler *Handler
	var ok bool
	if handler, ok = chaincodeSupport.chaincodeHasBeenLaunched(chaincode); !ok {
		chaincodeSupport.handlerMap.Unlock()
		chaincodeLog.Debug("handler not found for chaincode %s", chaincode)
		return fmt.Errorf("handler not found for chaincode %s", chaincode)
	}
	chaincodeSupport.handlerMap.Unlock()

	var notfy chan *pb.ChaincodeMessage
	var err error
	if notfy, err = handler.initOrReady(uuid, f, initArgs, tx, depTx); err != nil {
		return fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)
	}
	if notfy != nil {
		select {
		case ccMsg := <-notfy:
			if ccMsg.Type == pb.ChaincodeMessage_ERROR {
				err = fmt.Errorf("Error initializing container %s: %s", chaincode, string(ccMsg.Payload))
			}
		case <-time.After(timeout):
			err = fmt.Errorf("Timeout expired while executing send init message")
		}
	}

	//if initOrReady succeeded, our responsibility to delete the context
	handler.deleteTxContext(uuid)

	return err
}

//get args and env given chaincodeID
func (chaincodeSupport *ChaincodeSupport) getArgsAndEnv(cID *pb.ChaincodeID) (args []string, envs []string, err error) {
	envs = []string{"OPENCHAIN_CHAINCODE_ID_NAME=" + cID.Name}

	//chaincode executable will be same as the name of the chaincode
	args = []string{chaincodeSupport.chaincodeInstallPath + cID.Name, fmt.Sprintf("-peer.address=%s", chaincodeSupport.peerAddress)}

	chaincodeLog.Debug("Executable is %s", args[0])

	return args, envs, nil
}

// launchAndWaitForRegister will launch container if not already running
func (chaincodeSupport *ChaincodeSupport) launchAndWaitForRegister(context context.Context, cID *pb.ChaincodeID, uuid string) (bool, error) {
	chaincode := cID.Name
	if chaincode == "" {
		return false, fmt.Errorf("chaincode name not set")
	}

	chaincodeSupport.handlerMap.Lock()
	var ok bool
	//if its in the map, there must be a connected stream...nothing to do
	if _, ok = chaincodeSupport.chaincodeHasBeenLaunched(chaincode); ok {
		chaincodeLog.Debug("chaincode is running and ready: %s", chaincode)
		chaincodeSupport.handlerMap.Unlock()
		return true, nil
	}
	alreadyRunning := false
	notfy := chaincodeSupport.preLaunchSetup(chaincode)
	chaincodeSupport.handlerMap.Unlock()

	//launch the chaincode

	args, env, err := chaincodeSupport.getArgsAndEnv(cID)
	if err != nil {
		return alreadyRunning, err
	}

	//creat a StartImageReq obj and send it to VMCProcess
	vmname := container.GetVMFromName(chaincode)

	chaincodeLog.Debug("start container: %s", vmname)

	sir := container.StartImageReq{ID: vmname, Args: args, Env: env}
	resp, err := container.VMCProcess(context, "Docker", sir)
	if err != nil || (resp != nil && resp.(container.VMCResp).Err != nil) {
		if err == nil {
			err = resp.(container.VMCResp).Err
		}
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
	case <-time.After(chaincodeSupport.ccStartupTimeout):
		err = fmt.Errorf("Timeout expired while starting chaincode %s(tx:%s)", vmname, uuid)
	}
	if err != nil {
		chaincodeLog.Debug("stopping due to error while launching %s", err)
		errIgnore := chaincodeSupport.stopChaincode(context, cID)
		if errIgnore != nil {
			chaincodeLog.Debug("error on stop %s(%s)", errIgnore, err)
		}
	}
	return alreadyRunning, err
}

func (chaincodeSupport *ChaincodeSupport) stopChaincode(context context.Context, cID *pb.ChaincodeID) error {
	chaincode := cID.Name
	if chaincode == "" {
		return fmt.Errorf("chaincode name not set")
	}

	vmname := container.GetVMFromName(chaincode)

	//stop the chaincode
	sir := container.StopImageReq{ID: vmname, Timeout: 0}

	_, err := container.VMCProcess(context, "Docker", sir)
	if err != nil {
		err = fmt.Errorf("Error stopping container: %s", err)
		//but proceed to cleanup
	}

	chaincodeSupport.handlerMap.Lock()
	if _, ok := chaincodeSupport.chaincodeHasBeenLaunched(chaincode); !ok {
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
	chaincode := cID.Name
	chaincodeSupport.handlerMap.Lock()
	var handler *Handler
	var ok bool
	var err error
	//if its in the map, there must be a connected stream...nothing to do
	if handler, ok = chaincodeSupport.chaincodeHasBeenLaunched(chaincode); ok {
		if !handler.registered {
			chaincodeSupport.handlerMap.Unlock()
			chaincodeLog.Debug("premature execution - chaincode (%s) is being launched", chaincode)
			err = fmt.Errorf("premature execution - chaincode (%s) is being launched", chaincode)
			return cID, cMsg, err
		}
		if handler.isRunning() {
			chaincodeLog.Debug("chaincode is running(no need to launch) : %s", chaincode)
			chaincodeSupport.handlerMap.Unlock()
			return cID, cMsg, nil
		}
		chaincodeLog.Debug("Container not in READY state(%s)...send init/ready", handler.FSM.Current())
	}
	chaincodeSupport.handlerMap.Unlock()

	var depTx *pb.Transaction

	//extract depTx so we can initialize hander.deployTXSecContext
	//we need it only after container is launched and only if this is not a deploy tx
	//NOTE: ideally this section should be moved before just before sendInitOrReady where
	//      where we need depTx.  However, as we don't check for ExecuteTransactions failure
	//      in consensus/helper, the following race is not resolved:
	//         1) deploy creates image
	//         2) query launches chaincode
	//         3) deploy returns "premature execution" error
	//         4) error ignored and deploy committed
	//         5) query successfully retrives committed tx and calls sendInitOrReady
	// See issue #710

	if t.Type != pb.Transaction_CHAINCODE_NEW {
		ledger, ledgerErr := ledger.GetLedger()
		if ledgerErr != nil {
			return cID, cMsg, fmt.Errorf("Failed to get handle to ledger (%s)", ledgerErr)
		}

		//hopefully we are restarting from existing image and the deployed transaction exists
		depTx, ledgerErr = ledger.GetTransactionByUUID(chaincode)
		if ledgerErr != nil {
			return cID, cMsg, fmt.Errorf("Could not get deployment transaction for %s - %s", chaincode, ledgerErr)
		}
		if depTx == nil {
			return cID, cMsg, fmt.Errorf("deployment transaction does not exist for %s", chaincode)
		}
		if nil != chaincodeSupport.secHelper {
			var err error
			depTx, err = chaincodeSupport.secHelper.TransactionPreExecution(depTx)
			// Note that t is now decrypted and is a deep clone of the original input t
			if nil != err {
				return cID, cMsg, fmt.Errorf("failed tx preexecution%s - %s", chaincode, err)
			}
		}
	}

	//from here on : if we launch the container and get an error, we need to stop the container
	if !chaincodeSupport.userRunsCC && handler == nil {
		_, err = chaincodeSupport.launchAndWaitForRegister(context, cID, t.Uuid)
		if err != nil {
			chaincodeLog.Debug("launchAndWaitForRegister failed %s", err)
			return cID, cMsg, err
		}
	}

	if err == nil {
		//send init (if (f,args)) and wait for ready state
		err = chaincodeSupport.sendInitOrReady(context, t.Uuid, chaincode, f, initargs, chaincodeSupport.ccStartupTimeout, t, depTx)
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

	chaincodeLog.Debug("LaunchChaincode complete")

	return cID, cMsg, err
}

// getSecHelper returns the security help set from NewChaincodeSupport
func (chaincodeSupport *ChaincodeSupport) getSecHelper() crypto.Peer {
	return chaincodeSupport.secHelper
}

// DeployChaincode deploys the chaincode if not in development mode where user is running the chaincode.
func (chaincodeSupport *ChaincodeSupport) DeployChaincode(context context.Context, t *pb.Transaction) (*pb.ChaincodeDeploymentSpec, error) {
	if chaincodeSupport.userRunsCC {
		chaincodeLog.Debug("user runs chaincode, not deploying chaincode")
		return nil, nil
	}

	//build the chaincode
	cds := &pb.ChaincodeDeploymentSpec{}
	err := proto.Unmarshal(t.Payload, cds)
	if err != nil {
		return nil, err
	}
	cID := cds.ChaincodeSpec.ChaincodeID
	chaincode := cID.Name
	if err != nil {
		return cds, err
	}
	chaincodeSupport.handlerMap.Lock()
	//if its in the map, there must be a connected stream...and we are trying to build the code ?!
	if _, ok := chaincodeSupport.chaincodeHasBeenLaunched(chaincode); ok {
		chaincodeLog.Debug("deploy ?!! there's a chaincode with that name running: %s", chaincode)
		chaincodeSupport.handlerMap.Unlock()
		return cds, fmt.Errorf("deploy attempted but a chaincode with same name running %s", chaincode)
	}
	chaincodeSupport.handlerMap.Unlock()

	args, envs, err := chaincodeSupport.getArgsAndEnv(cID)
	if err != nil {
		return cds, fmt.Errorf("error getting args for chaincode %s", err)
	}

	vmname := container.GetVMFromName(chaincode)
	var targz io.Reader = bytes.NewBuffer(cds.CodePackage)
	cir := &container.CreateImageReq{ID: vmname, Args: args, Reader: targz, Env: envs}

	chaincodeLog.Debug("deploying chaincode %s", vmname)
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
func (chaincodeSupport *ChaincodeSupport) Execute(ctxt context.Context, chaincode string, msg *pb.ChaincodeMessage, timeout time.Duration, tx *pb.Transaction) (*pb.ChaincodeMessage, error) {
	chaincodeSupport.handlerMap.Lock()
	//we expect the chaincode to be running... sanity check
	handler, ok := chaincodeSupport.chaincodeHasBeenLaunched(chaincode)
	if !ok {
		chaincodeSupport.handlerMap.Unlock()
		chaincodeLog.Debug("cannot execute-chaincode is not running: %s", chaincode)
		return nil, fmt.Errorf("Cannot execute transaction or query for %s", chaincode)
	}
	chaincodeSupport.handlerMap.Unlock()

	var notfy chan *pb.ChaincodeMessage
	var err error
	if notfy, err = handler.sendExecuteMessage(msg, tx); err != nil {
		return nil, fmt.Errorf("Error sending %s: %s", msg.Type.String(), err)
	}
	var ccresp *pb.ChaincodeMessage
	select {
	case ccresp = <-notfy:
		if ccresp.Type == pb.ChaincodeMessage_ERROR || ccresp.Type == pb.ChaincodeMessage_QUERY_ERROR {
			err = fmt.Errorf(string(ccresp.Payload))
		}
	case <-time.After(timeout):
		err = fmt.Errorf("Timeout expired while executing transaction")
	}

	//our responsibility to delete transaction context if sendExecuteMessage succeeded
	handler.deleteTxContext(msg.Uuid)

	return ccresp, err
}

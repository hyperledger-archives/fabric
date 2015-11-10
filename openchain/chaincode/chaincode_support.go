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

	"github.com/op/go-logging"
	"github.com/golang/protobuf/proto"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	"github.com/openblockchain/obc-peer/openchain/container"
	pb "github.com/openblockchain/obc-peer/protos"
)

var chainletLog = logging.MustGetLogger("chaincode")

type ChainName string
const (
	DEFAULTCHAIN ChainName = "default"
	USERRUNSCHAINCODE string = "user_runs_chaincode"
)

//this needs to be a first class, top-level object... for now, lets just have a placeholder
var chains map[ChainName]*ChainletSupport
var CC_STARTUP_TIMEOUT time.Duration
var USER_RUNS_CC bool

func init() {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigName("openchain") // name of config file (without extension)
	viper.AddConfigPath("./")  // path to look for the config file in
	err := viper.ReadInConfig()      // Find and read the config file
	if err != nil {
		fmt.Printf("Error reading viper :%s\n", err)
	}

	chains = make(map[ChainName]*ChainletSupport)
	to,err := strconv.Atoi(viper.GetString("chainlet.startuptimeout"))
	if err != nil { //what went wrong ?
		fmt.Printf("could not retrive timeout var...setting to 5secs\n")
		to = 5000
	}
	CC_STARTUP_TIMEOUT = time.Duration(to)*time.Millisecond

	mode := viper.GetString("validator.chaincoderunmode")
	if mode == USERRUNSCHAINCODE {
		USER_RUNS_CC = true
	} else {
		USER_RUNS_CC = false
	}
	fmt.Printf("chainlet env using [startuptimeout-%d, chaincode run mode-%s]\n", to, mode)
}

type handlerMap struct {
	sync.RWMutex
	m map[string]*Handler
}

func GetChain(name ChainName) *ChainletSupport {
	return chains[name]
}

// NewChainletSupport Creates a new ChainletSupport instance
func NewChainletSupport() *ChainletSupport {
	//we need to pass chainname when we do multiple chains...till then use DEFAULTCHAIN
	s := &ChainletSupport{ name: DEFAULTCHAIN, handlerMap: &handlerMap{m: make(map[string]*Handler)}}

	//initialize global chain
	chains[DEFAULTCHAIN] = s

	return s
}

// // ChaincodeStream standard stream for ChaincodeMessage type.
// type ChaincodeStream interface {
// 	Send(*pb.ChaincodeMessage) error
// 	Recv() (*pb.ChaincodeMessage, error)
// }

// ChainletSupport responsible for providing interfacing with chaincodes from the Peer.
type ChainletSupport struct {
	name		ChainName
	handlerMap *handlerMap
}

// DuplicateChaincodeHandlerError returned if attempt to register same chaincodeID while a stream already exists.
type DuplicateChaincodeHandlerError struct {
	ChaincodeID *pb.ChainletID
}

func (d *DuplicateChaincodeHandlerError) Error() string {
	return fmt.Sprintf("Duplicate chaincodeID error: %s", d.ChaincodeID)
}

func newDuplicateChaincodeHandlerError(chaincodeHandler *Handler) error {
	return &DuplicateChaincodeHandlerError{ChaincodeID: chaincodeHandler.ChaincodeID}
}

func getHandlerKey(cID *pb.ChainletID) (string, error) {
	if cID == nil {
		return "", fmt.Errorf("Could not find chaincode handler with nil ChaincodeID")
	}
	return cID.Url + ":" + cID.Version, nil
}

func (c *ChainletSupport) registerHandler(chaincodehandler *Handler) error {
	key, err := getHandlerKey(chaincodehandler.ChaincodeID)
	if err != nil {
		return fmt.Errorf("Error registering handler: %s", err)
	}
	c.handlerMap.Lock()
	defer c.handlerMap.Unlock()

	var h2 *Handler
	if h2, ok := c.handlerMap.m[key]; ok == true && h2.registered == true {
	        chaincodeLogger.Debug("duplicate registered handler(key:%s) return error", key)
		// Duplicate, return error
		return newDuplicateChaincodeHandlerError(chaincodehandler)
	}
        //a placeholder, unregistered handler will be setup by query or transaction processing that comes
	//through via consensus. In this case we swap the handler and give it the notify channel 
	if h2 != nil {
		chaincodehandler.readyNotify = h2.readyNotify
		delete(c.handlerMap.m, key)
	}

	c.handlerMap.m[key] = chaincodehandler

	chaincodehandler.registered = true

	//now we are ready to receive messages and send back responses
	chaincodehandler.responseNotifiers = make (map[string]chan *pb.ChaincodeMessage)

	chaincodeLogger.Debug("registered handler complete for chaincode %s", key)

	return nil
}

func (c *ChainletSupport) deregisterHandler(chaincodehandler *Handler) error {
	key, err := getHandlerKey(chaincodehandler.ChaincodeID)
	if err != nil {
		return fmt.Errorf("Error deregistering handler: %s", err)
	}
	c.handlerMap.Lock()
	defer c.handlerMap.Unlock()
	if _, ok := c.handlerMap.m[key]; !ok {
		// Handler NOT found
		return fmt.Errorf("Error deregistering handler, could not find handler with key: %s", key)
	}
	delete(c.handlerMap.m, key)
	chaincodeLogger.Debug("Deregistered handler with key: %s", key)
	return nil
}

// GetExecutionContext returns the execution context.  DEPRECATED. TO be removed.
func (c *ChainletSupport) GetExecutionContext(context context.Context, requestContext *pb.ChainletRequestContext) (*pb.ChainletExecutionContext, error) {
	//chainletId := &pb.ChainletIdentifier{Url: "github."}
	timeStamp := &google_protobuf.Timestamp{Seconds: time.Now().UnixNano(), Nanos: 0}
	executionContext := &pb.ChainletExecutionContext{ChainletId: requestContext.GetId(),
		Timestamp: timeStamp}

	chainletLog.Debug("returning execution context: %s", executionContext)
	return executionContext, nil
}

func(c *ChainletSupport) sendInitOrReady(context context.Context, uuid string, chaincode string, f *string, initArgs []string, timeout time.Duration) error {
	c.handlerMap.Lock()
	//if its in the map, there must be a connected stream...nothing to do
	var handler *Handler
	var ok bool
	if handler, ok = c.handlerMap.m[chaincode]; !ok {
		c.handlerMap.Unlock()
		chainletLog.Debug("[sendInitOrRead]handler not found for chaincode %s", chaincode)
		return fmt.Errorf("handler not found for chaincode %s", chaincode)
	}
	c.handlerMap.Unlock()

	var notfy chan *pb.ChaincodeMessage
	var err error
	if notfy,err = handler.initOrReady(uuid, f, initArgs); err != nil {
		return fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)
	}
	if notfy != nil {
		select {
		case ccMsg := <- notfy:
			if ccMsg.Type == pb.ChaincodeMessage_ERROR {
				return fmt.Errorf("Error initializing container %s: %s", chaincode, string(ccMsg.Payload) )
			}
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("Timeout expired while executing send init message")
		}
	}
	return err
}

func (c *ChainletSupport) launchAndWaitForRegister(context context.Context, chaincode string, uuid string) (bool,error) {
	c.handlerMap.Lock()

	var alreadyRunning bool = true
	//if its in the map, there must be a connected stream...nothing to do
	if _, ok := c.handlerMap.m[chaincode]; ok {
		c.handlerMap.Unlock()
		chainletLog.Debug("[LaunchChainCode]chaincode is running: %s", chaincode)
		return alreadyRunning,nil
	}

	alreadyRunning = false
	//register placeholder Handler. This will be transferred in registerHandler
	notfy := make(chan bool, 1)
	c.handlerMap.m[chaincode] = &Handler{ readyNotify: notfy }

	c.handlerMap.Unlock()

	//launch the chaincode
	//creat a StartImageReq obj and send it to VMCProcess
	sir := container.StartImageReq{ID: chaincode}
	_, err := container.VMCProcess(context, "Docker", sir)
	if err != nil {
        	err = fmt.Errorf("Error starting container: %s", err)
		c.handlerMap.Lock()
		delete(c.handlerMap.m, chaincode)
		c.handlerMap.Unlock()
		return alreadyRunning,err
	}

	//wait for REGISTER state
	select {
	case ok := <-notfy:
		if !ok {
			err = fmt.Errorf("registration failed for %s(tx:%s)", chaincode, uuid)
		}
	case <-time.After(CC_STARTUP_TIMEOUT):
		err = fmt.Errorf("Timeout expired while starting chaincode %s(tx:%s)", chaincode, uuid)
	}
	if err != nil {
		//TODO stop the container
	}
	return alreadyRunning, err
}

//LaunchChainCode - will launch the chaincode if not running (if running return nil) and will wait for
//handler for the chaincode to get into FSM (ready state)
func (c *ChainletSupport) LaunchChaincode(context context.Context, t *pb.Transaction) (*pb.ChainletID, *pb.ChainletMessage, error) {
	//build the chaincode
	var cID *pb.ChainletID
	var cMsg *pb.ChainletMessage
	var f *string
	var initargs []string
	if t.Type == pb.Transaction_CHAINLET_NEW {
		cds := &pb.ChainletDeploymentSpec{}
		err := proto.Unmarshal(t.Payload, cds)
		if err != nil {
			return nil,nil,err
		}
		cID = cds.ChainletSpec.ChainletID
		cMsg = cds.ChainletSpec.CtorMsg
		f = &cMsg.Function
		initargs = cMsg.Args
	} else if t.Type == pb.Transaction_CHAINLET_EXECUTE || t.Type == pb.Transaction_CHAINLET_QUERY {
		ci := &pb.ChaincodeInvocation {}
		err := proto.Unmarshal(t.Payload, ci)
		if err != nil {
			return nil,nil,err
		}
		cID = ci.ChainletSpec.ChainletID
		cMsg = ci.Message
	} else {
		c.handlerMap.Unlock()
	        return nil,nil,fmt.Errorf("invalid transaction type: %d", t.Type)
	}
	chaincode, err := getHandlerKey(cID)
	if err != nil {
		return cID,cMsg,err
	}

	//from here on : if we launch the container and get an error, we need to stop the container
	if !USER_RUNS_CC  {
		vmname, err := container.GetVMName(cID)
		alreadyRunning,err := c.launchAndWaitForRegister(context, vmname, t.Uuid)
		if err != nil {
			return cID,cMsg,err
		}
		if alreadyRunning {
			return cID,cMsg,nil
		}
	}

	if err == nil {
		//send init (if (f,args)) and wait for ready state
		err = c.sendInitOrReady(context, t.Uuid, chaincode, f, initargs, CC_STARTUP_TIMEOUT)
		if err != nil {
			err = fmt.Errorf("Failed to init chaincode(%s)", err)
		}
	}

	if !USER_RUNS_CC && err != nil {
		//TODO stop container
	}
	return cID,cMsg,err
}

func (c *ChainletSupport) DeployChaincode(context context.Context, t *pb.Transaction) (*pb.ChainletDeploymentSpec, error) {
	if USER_RUNS_CC {
		chainletLog.Debug("command line chaincode won't deploy")
		return nil, nil
	}
	//build the chaincode
	cds := &pb.ChainletDeploymentSpec{}
	err := proto.Unmarshal(t.Payload, cds)
	if err != nil {
		return nil, err
	}
	chaincode, err := container.GetVMName(cds.ChainletSpec.ChainletID)
	if err != nil {
		return cds, err
	}
	c.handlerMap.Lock()
	//if its in the map, there must be a connected stream...and we are trying to build the code ?!
	if _, ok := c.handlerMap.m[chaincode]; ok {
		chainletLog.Debug("deploy ?!! there's a chaincode with that name running: %s", chaincode)
		c.handlerMap.Unlock()
		return cds, nil
	}
	c.handlerMap.Unlock()

	//create build request ...
	var targz io.Reader = bytes.NewBuffer(cds.CodePackage)
	cir := &container.CreateImageReq{ID: chaincode, Reader: targz}

	//create image and create container
	_, err = container.VMCProcess(context, "Docker", cir)
	if err != nil {
	        err = fmt.Errorf("Error starting container: %s", err)
	}

	return cds, err
}

func (c *ChainletSupport) SendTransaction(context context.Context, t *pb.Transaction) (*pb.ChainletDeploymentSpec, error) {
	//build the chaincode
	cds := &pb.ChainletDeploymentSpec{}
	err := proto.Unmarshal(t.Payload, cds)
	if err != nil {
		return nil, err
	}
	chaincode, err := container.GetVMName(cds.ChainletSpec.ChainletID)
	if err != nil {
		return cds, err
	}
	c.handlerMap.Lock()
	//if its in the map, there must be a connected stream...and we are trying to build the code ?!
	if _, ok := c.handlerMap.m[chaincode]; ok {
		chainletLog.Debug("deploy ?!! there's a chaincode with that name running: %s", chaincode)
		c.handlerMap.Unlock()
		return cds, nil
	}
	c.handlerMap.Unlock()

	//create build request ...
	var targz io.Reader = bytes.NewBuffer(cds.CodePackage)
	cir := &container.CreateImageReq{ID: chaincode, Reader: targz}

	//create image and create container
	_, err = container.VMCProcess(context, "Docker", cir)
	if err != nil {
	        err = fmt.Errorf("Error starting container: %s", err)
	}

	return cds, err
}
// Register the bidi stream entry point called by chaincode to register with the Peer.
func (c *ChainletSupport) Register(stream pb.ChainletSupport_RegisterServer) error {
	return HandleChaincodeStream(c, stream)
}

func CreateTransactionMessage(uuid string, cMsg *pb.ChainletMessage) (*pb.ChaincodeMessage,error) {
	payload, err := proto.Marshal(cMsg)
	if err != nil {
		return nil,err
	}
	return &pb.ChaincodeMessage { Type: pb.ChaincodeMessage_TRANSACTION, Payload: payload, Uuid: uuid }, nil
}

func CreateQueryMessage(uuid string, cMsg *pb.ChainletMessage) (*pb.ChaincodeMessage,error) {
	payload, err := proto.Marshal(cMsg)
	if err != nil {
		return nil,err
	}
	return &pb.ChaincodeMessage { Type: pb.ChaincodeMessage_QUERY, Payload: payload, Uuid: uuid }, nil
}

func (c* ChainletSupport) Execute(ctxt context.Context, chaincode string, msg *pb.ChaincodeMessage, timeout time.Duration) (*pb.ChaincodeMessage, error) {
	c.handlerMap.Lock()
	//if its in the map, there must be a connected stream...nothing to do
	handler,ok := c.handlerMap.m[chaincode]
	if !ok {
		c.handlerMap.Unlock()
		chainletLog.Debug("[Execute]chaincode is not running: %s", chaincode)
		return nil, fmt.Errorf("Cannot execute transaction or query for %s", chaincode)
	}
	c.handlerMap.Unlock()
	
	var notfy chan *pb.ChaincodeMessage
	var err error
	if notfy,err = handler.SendMessage(msg); err != nil {
		return nil, fmt.Errorf("Error sending %s: %s", pb.ChaincodeMessage_INIT, err)
	}
	select {
	case ccresp := <- notfy:
		return ccresp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("Timeout expired while executing transaction")
	}
}

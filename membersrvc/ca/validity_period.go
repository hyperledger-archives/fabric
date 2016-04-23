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

package ca

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	obc "github.com/hyperledger/fabric/protos"
)

const (
	//TODO: this values could be configurable through a configuration file for system chaincode
	systemChaincodeTimeout               = time.Second * 3
	drjTime                time.Duration = 37
	function                             = "invoke"
)

var (
	chaincodeInvocation            *obc.ChaincodeInvocationSpec
	invocationErrorAlreadyReported = false
	initialized                    = false
)

func initialize() {
	if !initialized {
		//TODO: this should be the login token for the component in charge of the validity period update.
		//This component needs to be registered in the system to be able to invoke the update validity period system chaincode.
		token := "system_chaincode_invoker"

		chaincodeInvocation = createChaincodeInvocation(strconv.FormatInt(time.Now().Unix(), 10), token)
		initialized = true
	}
}

func updateValidityPeriod() {
	initialize()

	for {
		chaincodeInvocation.ChaincodeSpec.CtorMsg.Args[0] = strconv.FormatInt(time.Now().Unix(), 10)

		err := invokeChaincode(chaincodeInvocation)
		if err != nil && !invocationErrorAlreadyReported {
			Error.Printf("Error while updating validity period. Error was: %s", err)
			invocationErrorAlreadyReported = true
		} else {
			invocationErrorAlreadyReported = false
		}

		time.Sleep(time.Second * drjTime)
	}
}

func getDevopsClient(peerAddress string) (obc.DevopsClient, error) {
	var opts []grpc.DialOption
	if viper.GetBool("pki.validity-period.tls.enabled") {
		var sn string
		if viper.GetString("pki.validity-period.tls.serverhostoverride") != "" {
			sn = viper.GetString("pki.validity-period.tls.serverhostoverride")
		}
		var creds credentials.TransportAuthenticator
		if viper.GetString("pki.validity-period.tls.cert.file") != "" {
			var err error
			creds, err = credentials.NewClientTLSFromFile(viper.GetString("pki.validity-period.tls.cert.file"), sn)
			if err != nil {
				grpclog.Fatalf("Failed to create TLS credentials %v", err)
			}
		} else {
			creds = credentials.NewClientTLSFromCert(nil, sn)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}
	opts = append(opts, grpc.WithTimeout(systemChaincodeTimeout))
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithInsecure())
	conn, err := grpc.Dial(peerAddress, opts...)

	if err != nil {
		return nil, fmt.Errorf("Error trying to connect to local peer: %s", err)
	}

	devopsClient := obc.NewDevopsClient(conn)
	return devopsClient, nil
}

func invokeChaincode(chaincodeInvSpec *obc.ChaincodeInvocationSpec) error {

	devopsClient, err := getDevopsClient(viper.GetString("pki.validity-period.devops-address"))
	if err != nil {
		Error.Println(fmt.Sprintf("Error retrieving devops client: %s", err))
		return err
	}

	resp, err := devopsClient.Invoke(context.Background(), chaincodeInvSpec)

	if err != nil {
		Error.Println(fmt.Sprintf("Error invoking validity period update system chaincode: %s", err))
		return err
	}

	Info.Println("Successfully invoked validity period update: %s(%s)", chaincodeInvSpec, string(resp.Msg))

	return nil
}

func createChaincodeInvocation(validityPeriod string, token string) *obc.ChaincodeInvocationSpec {
	spec := &obc.ChaincodeSpec{Type: obc.ChaincodeSpec_GOLANG,
		ChaincodeID: &obc.ChaincodeID{Name: viper.GetString("pki.validity-period.chaincodeHash")},
		CtorMsg: &obc.ChaincodeInput{Function: function,
			Args: []string{validityPeriod},
		},
	}

	spec.SecureContext = string(token)

	invocationSpec := &obc.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	return invocationSpec
}

func validityPeriodUpdateEnabled() bool {
	// If the update of the validity period is enabled in the configuration file return the configured value
	if viper.IsSet("pki.validity-period.update") {
		return viper.GetBool("pki.validity-period.update")
	}

	// Validity period update is enabled by default if no configuration was specified.
	return true
}

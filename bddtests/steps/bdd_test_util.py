
# Copyright IBM Corp. 2016 All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

import os
import re
import subprocess
import devops_pb2
import fabric_pb2
import chaincode_pb2

from grpc.beta import implementations

def cli_call(context, arg_list, expect_success=True):
    """Executes a CLI command in a subprocess and return the results.

    @param context: the behave context
    @param arg_list: a list command arguments
    @param expect_success: use False to return even if an error occurred when executing the command
    @return: (string, string, int) output message, error message, return code
    """
    #arg_list[0] = "update-" + arg_list[0]

    # We need to run the cli command by actually calling the python command
    # the update-cli.py script has a #!/bin/python as the first line
    # which calls the system python, not the virtual env python we
    # setup for running the update-cli
    p = subprocess.Popen(arg_list, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    output, error = p.communicate()
    if p.returncode != 0:
        if output is not None:
            print("Output:\n" + output)
        if error is not None:
            print("Error Message:\n" + error)
        if expect_success:
            raise subprocess.CalledProcessError(p.returncode, arg_list, output)
    return output, error, p.returncode

class UserRegistration:
    def __init__(self, secretMsg, composeService):
        self.secretMsg = secretMsg
        self.composeService = composeService   
        self.tags = {}   
        self.lastResult = None  

    def getUserName(self):
        return self.secretMsg['enrollId']

    def getSecret(self):
        return devops_pb2.Secret(enrollId=self.secretMsg['enrollId'],enrollSecret=self.secretMsg['enrollSecret'])


# Registerses a user on a specific composeService
def registerUser(context, secretMsg, composeService):
    userName = secretMsg['enrollId']
    if 'users' in context:
        pass
    else:
        context.users = {}
    if userName in context.users:
        raise Exception("User already registered: {0}".format(userName)) 
    context.users[userName] = UserRegistration(secretMsg, composeService) 

# Registerses a user on a specific composeService
def getUserRegistration(context, enrollId):
    userRegistration = None
    if 'users' in context:
        pass
    else:
        context.users = {}
    if enrollId in context.users:
        userRegistration = context.users[enrollId] 
    else:
        raise Exception("User has not been registered: {0}".format(enrollId)) 
    return userRegistration

    
def ipFromContainerNamePart(namePart, containerDataList):
    """Returns the IPAddress based upon a name part of the full container name"""
    ip = None
    containerNamePrefix = os.path.basename(os.getcwd()) + "_"
    for containerData in containerDataList:
        if containerData.containerName.startswith(containerNamePrefix + namePart):
            ip = containerData.ipAddress
    if ip == None:
        raise Exception("Could not find container with namePart = {0}".format(namePart))
    return ip

def getTxResult(context, enrollId):
    '''Returns the TransactionResult using the enrollId supplied'''
    assert 'users' in context, "users not found in context. Did you register a user?"
    assert 'compose_containers' in context, "compose_containers not found in context"

    (channel, userRegistration) = getGRPCChannelAndUser(context, enrollId)
    stub = devops_pb2.beta_create_Devops_stub(channel)
    
    txRequest = devops_pb2.TransactionRequest(transactionUuid = context.transactionID)
    response = stub.GetTransactionResult(txRequest, 2)
    assert response.status == fabric_pb2.Response.SUCCESS, 'Failure getting Transaction Result from {0}, for user "{1}":  {2}'.format(userRegistration.composeService,enrollId, response.msg)
    # Now grab the TransactionResult from the Msg bytes
    txResult = fabric_pb2.TransactionResult()
    txResult.ParseFromString(response.msg)
    return txResult

def getGRPCChannel(ipAddress):
    channel = implementations.insecure_channel(ipAddress, 30303)
    print("Returning GRPC for address: {0}".format(ipAddress))
    return channel

def getGRPCChannelAndUser(context, enrollId):
    '''Returns a tuple of GRPC channel and UserRegistration instance.  The channel is open to the composeService that the user registered with.'''
    userRegistration = getUserRegistration(context, enrollId)

    # Get the IP address of the server that the user registered on
    ipAddress = ipFromContainerNamePart(userRegistration.composeService, context.compose_containers)

    channel = getGRPCChannel(ipAddress)

    return (channel, userRegistration) 


def getDeployment(context, ccAlias):
    '''Return a deployment with chaincode alias from prior deployment, or None if not found'''
    deployment = None
    if 'deployments' in context:
        pass
    else:
        context.deployments = {}
    if ccAlias in context.deployments:
        deployment = context.deployments[ccAlias] 
    # else:
    #     raise Exception("Deployment alias not found: '{0}'.  Are you sure you have deployed a chaincode with this alias?".format(ccAlias)) 
    return deployment

def deployChaincode(context, enrollId, chaincodePath, ccAlias, ctor):
    '''Deploy a chaincode with the specified alias for the specfied enrollId'''
    (channel, userRegistration) = getGRPCChannelAndUser(context, enrollId)
    stub = devops_pb2.beta_create_Devops_stub(channel)

    # Make sure deployment alias does NOT already exist
    assert getDeployment(context, ccAlias) == None, "Deployment alias already exists: '{0}'.".format(ccAlias)

    args = getArgsFromContextForUser(context, enrollId)
    ccSpec = chaincode_pb2.ChaincodeSpec(type = chaincode_pb2.ChaincodeSpec.GOLANG,
        chaincodeID = chaincode_pb2.ChaincodeID(name="",path=chaincodePath),
        ctorMsg = chaincode_pb2.ChaincodeInput(function = ctor, args = args))
    ccSpec.secureContext = userRegistration.getUserName()
    if 'metadata' in context:
        ccSpec.metadata = context.metadata
    try:
        ccDeploymentSpec = stub.Deploy(ccSpec, 60)
        ccSpec.chaincodeID.name = ccDeploymentSpec.chaincodeSpec.chaincodeID.name
        context.grpcChaincodeSpec = ccSpec
        context.deployments[ccAlias] = ccSpec
    except:
        del stub
        raise

def invokeChaincode(context, enrollId, ccAlias, functionName):
    # Get the deployment for the supplied chaincode alias
    deployedCcSpec = getDeployment(context, ccAlias)
    assert deployedCcSpec != None, "Deployment NOT found for chaincode alias '{0}'".format(ccAlias)

    # Create a new ChaincodeSpec by copying the deployed one
    newChaincodeSpec = chaincode_pb2.ChaincodeSpec()
    newChaincodeSpec.CopyFrom(deployedCcSpec)
    
    # Update hte chaincodeSpec ctorMsg for invoke
    args = getArgsFromContextForUser(context, enrollId)
    
    chaincodeInput = chaincode_pb2.ChaincodeInput(function = functionName, args = args ) 
    newChaincodeSpec.ctorMsg.CopyFrom(chaincodeInput)

    ccInvocationSpec = chaincode_pb2.ChaincodeInvocationSpec(chaincodeSpec = newChaincodeSpec)

    (channel, userRegistration) = getGRPCChannelAndUser(context, enrollId)
    
    stub = devops_pb2.beta_create_Devops_stub(channel)
    response = stub.Invoke(ccInvocationSpec,2)
    return response

def getArgsFromContextForUser(context, enrollId):
    # Update the chaincodeSpec ctorMsg for invoke
    args = []
    if 'table' in context:
       # There are function arguments
       userRegistration = getUserRegistration(context, enrollId)
       # Allow the user to specify expressions referencing tags in the args list
       pattern = re.compile('\{(.*)\}$')
       for arg in context.table[0].cells:
          m = pattern.match(arg)
          if m:
              # tagName reference found in args list
              tagName = m.groups()[0]
              # make sure the tagName is found in the users tags
              assert tagName in userRegistration.tags, "TagName '{0}' not found for user '{1}'".format(tagName, userRegistration.getUserName())
              args.append(userRegistration.tags[tagName])
          else:
              #No tag referenced, pass the arg
              args.append(arg)
    return args




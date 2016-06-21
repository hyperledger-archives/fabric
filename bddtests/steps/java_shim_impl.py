#
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
import time
import copy
from datetime import datetime, timedelta

import sys, requests, json

import bdd_test_util

CORE_REST_PORT = 5000

class ContainerData:
    def __init__(self, containerName, ipAddress, envFromInspect, composeService):
        self.containerName = containerName
        self.ipAddress = ipAddress
        self.envFromInspect = envFromInspect
        self.composeService = composeService

    def getEnv(self, key):
        envValue = None
        for val in self.envFromInspect:
            if val.startswith(key):
                envValue = val[len(key):]
                break
        if envValue == None:
            raise Exception("ENV key not found ({0}) for container ({1})".format(key, self.containerName))
        return envValue

def parseComposeOutput(context):
    """Parses the compose output results and set appropriate values into context.  Merges existing with newly composed."""
    # Use the prefix to get the container name
    containerNamePrefix = os.path.basename(os.getcwd()) + "_"
    containerNames = []
    for l in context.compose_error.splitlines():
        tokens = l.split()
        print(tokens)
        if 1 < len(tokens):
            thisContainer = tokens[1]
            if containerNamePrefix not in thisContainer:
                thisContainer = containerNamePrefix + thisContainer + "_1"
            if thisContainer not in containerNames:
                containerNames.append(thisContainer)

    print("Containers started: ")
    print(containerNames)
    # Now get the Network Address for each name, and set the ContainerData onto the context.
    containerDataList = []
    for containerName in containerNames:
        output, error, returncode = \
            bdd_test_util.cli_call(context, ["docker", "inspect", "--format",  "{{ .NetworkSettings.IPAddress }}", containerName], expect_success=True)
        #print("container {0} has address = {1}".format(containerName, output.splitlines()[0]))
        ipAddress = output.splitlines()[0]

        # Get the environment array
        output, error, returncode = \
            bdd_test_util.cli_call(context, ["docker", "inspect", "--format",  "{{ .Config.Env }}", containerName], expect_success=True)
        env = output.splitlines()[0][1:-1].split()

        # Get the Labels to access the com.docker.compose.service value
        output, error, returncode = \
            bdd_test_util.cli_call(context, ["docker", "inspect", "--format",  "{{ .Config.Labels }}", containerName], expect_success=True)
        labels = output.splitlines()[0][4:-1].split()
        dockerComposeService = [composeService[27:] for composeService in labels if composeService.startswith("com.docker.compose.service:")][0]
        print("dockerComposeService = {0}".format(dockerComposeService))
        print("container {0} has env = {1}".format(containerName, env))
        containerDataList.append(ContainerData(containerName, ipAddress, env, dockerComposeService))
    # Now merge the new containerData info with existing
    newContainerDataList = []
    if "compose_containers" in context:
        # Need to merge I new list
        newContainerDataList = context.compose_containers
    newContainerDataList = newContainerDataList + containerDataList

    setattr(context, "compose_containers", newContainerDataList)
    print("")

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

def buildUrl(context, ipAddress, path):
    schema = "http"
    if 'TLS' in context.tags:
        schema = "https"
    return "{0}://{1}:{2}{3}".format(schema, ipAddress, CORE_REST_PORT, path)

def currentTime():
    return time.strftime("%H:%M:%S")

def getDockerComposeFileArgsFromYamlFile(compose_yaml):
    parts = compose_yaml.split()
    args = []
    for part in parts:
        args = args + ["-f"] + [part]
    return args

@when(u'I deploy chaincode "{chaincodePath}" of "{chainLang}" with ctor "{ctor}" to "{containerName}"')
def step_impl(context, chaincodePath, chainLang, ctor, containerName):
    print("Printing chaincode language " + chainLang)
    ipAddress = ipFromContainerNamePart(containerName, context.compose_containers)
    request_url = buildUrl(context, ipAddress, "/devops/deploy")
    print("Requesting path = {0}".format(request_url))
    args = []
    if 'table' in context:
       # There is ctor arguments
       args = context.table[0].cells
    #typeGolang =

    # Create a ChaincodeSpec structure
    chaincodeSpec = {
        "type": chainLang,
        "chaincodeID": {
            "path" : chaincodePath,
            "name" : ""
        },
        "ctorMsg":  {
            "function" : ctor,
            "args" : args
        },
    }
    if 'userName' in context:
        chaincodeSpec["secureContext"] = context.userName

    resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeSpec), verify=False)
    assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
    context.response = resp
    chaincodeName = resp.json()['message']
    chaincodeSpec['chaincodeID']['name'] = chaincodeName
    context.chaincodeSpec = chaincodeSpec
    print(json.dumps(chaincodeSpec, indent=4))
    print("")

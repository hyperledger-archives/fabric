import os
import re
import time
import copy
from datetime import datetime, timedelta

import sys, requests, json

import bdd_test_util

OPENCHAIN_REST_PORT = 5000

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
    	print(l.split())
    	containerNames.append(l.split()[1])

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

def buildUrl(ipAddress, path):
	return "http://{0}:{1}{2}".format(ipAddress, OPENCHAIN_REST_PORT, path)

@given(u'we compose "{composeYamlFile}"')
def step_impl(context, composeYamlFile):
	# Use the uninstalled version of `cf active-deploy` rather than the installed version on the OS $PATH
    #cmd = os.path.dirname(os.path.abspath(__file__)) + "/../../../cf_update/v1/cf_update.py"

    # Expand $vars, e.g. "--path $PATH" becomes "--path /bin"
    #args = re.sub('\$\w+', lambda v: os.getenv(v.group(0)[1:]), composeYamlFile)
    context.compose_yaml = composeYamlFile
    context.compose_output, context.compose_error, context.compose_returncode = \
        bdd_test_util.cli_call(context, ["docker-compose", "-f", composeYamlFile, "up","--force-recreate", "-d"], expect_success=True)
    assert context.compose_returncode == 0, "docker-compose failed to bring up {0}".format(composeYamlFile)
    parseComposeOutput(context)

@when(u'requesting "{path}" from "{containerName}"')
def step_impl(context, path, containerName):
    ipAddress = ipFromContainerNamePart(containerName, context.compose_containers)
    request_url = buildUrl(ipAddress, path)
    print("Requesting path = {0}".format(request_url))
    resp = requests.get(request_url, headers={'Accept': 'application/json'})
    assert resp.status_code == 200, "Failed to GET url %s:  %s" % (request_url,resp.text)
    context.response = resp
    print("")

@then(u'I should get a JSON response with "{attribute}" = "{expectedValue}"')
def step_impl(context, attribute, expectedValue):
    assert attribute in context.response.json(), "Attribute not found in response (%s)" %(attribute)
    foundValue = context.response.json()[attribute]
    assert (str(foundValue) == expectedValue), "For attribute %s, expected (%s), instead found (%s)" % (attribute, expectedValue, foundValue)


@given(u'I wait "{seconds}" seconds')
def step_impl(context, seconds):
    time.sleep(float(seconds))

@when(u'I wait "{seconds}" seconds')
def step_impl(context, seconds):
    time.sleep(float(seconds))

@then(u'I wait "{seconds}" seconds')
def step_impl(context, seconds):
    time.sleep(float(seconds))


@when(u'I deploy chaincode "{chaincodePath}" with ctor "{ctor}" to "{containerName}"')
def step_impl(context, chaincodePath, ctor, containerName):
    ipAddress = ipFromContainerNamePart(containerName, context.compose_containers)
    request_url = buildUrl(ipAddress, "/devops/deploy")
    print("Requesting path = {0}".format(request_url))
    args = []
    if 'table' in context:
	   # There is ctor arguments
	   args = context.table[0].cells
    typeGolang = 1

    # Create a ChaincodeSpec structure
    chaincodeSpec = {
        "type": typeGolang,
        "chaincodeID": {
            "path" : chaincodePath,
            "name" : ""
        },
        "ctorMsg":  {
            "function" : ctor,
            "args" : args
        },
        #"secureContext" : "binhn"
    }
    if 'userName' in context:
        chaincodeSpec["secureContext"] = context.userName

    resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeSpec))
    assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
    context.response = resp
    chaincodeName = resp.json()['message']
    chaincodeSpec['chaincodeID']['name'] = chaincodeName
    context.chaincodeSpec = chaincodeSpec
    print(json.dumps(chaincodeSpec, indent=4))
    print("")

@then(u'I should have received a chaincode name')
def step_impl(context):
    if 'chaincodeSpec' in context:
        assert context.chaincodeSpec['chaincodeID']['name'] != ""
        # Set the current transactionID to the name passed back
        context.transactionID = context.chaincodeSpec['chaincodeID']['name']
    else:
        fail('chaincodeSpec not in context')

@when(u'I invoke chaincode "{chaincodeName}" function name "{functionName}" on "{containerName}" "{times}" times')
def step_impl(context, chaincodeName, functionName, containerName, times):
    assert 'chaincodeSpec' in context, "chaincodeSpec not found in context"
    for i in range(int(times)):
        invokeChaincode(context, "invoke", functionName, containerName)

@when(u'I invoke chaincode "{chaincodeName}" function name "{functionName}" on "{containerName}"')
def step_impl(context, chaincodeName, functionName, containerName):
    assert 'chaincodeSpec' in context, "chaincodeSpec not found in context"
    invokeChaincode(context, "invoke", functionName, containerName)

@then(u'I should have received a transactionID')
def step_impl(context):
    assert 'transactionID' in context, 'transactionID not found in context'
    assert context.transactionID != ""
    pass

@when(u'I query chaincode "{chaincodeName}" function name "{functionName}" on "{containerName}"')
def step_impl(context, chaincodeName, functionName, containerName):
    invokeChaincode(context, "query", functionName, containerName)

def invokeChaincode(context, devopsFunc, functionName, containerName):
    assert 'chaincodeSpec' in context, "chaincodeSpec not found in context"
    # Update hte chaincodeSpec ctorMsg for invoke
    args = []
    if 'table' in context:
       # There is ctor arguments
       args = context.table[0].cells
    context.chaincodeSpec['ctorMsg']['function'] = functionName
    context.chaincodeSpec['ctorMsg']['args'] = args
    # Invoke the POST
    chaincodeInvocationSpec = {
        "chaincodeSpec" : context.chaincodeSpec
    }
    ipAddress = ipFromContainerNamePart(containerName, context.compose_containers)
    request_url = buildUrl(ipAddress, "/devops/{0}".format(devopsFunc))
    print("POSTing path = {0}".format(request_url))

    resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeInvocationSpec))
    assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
    context.response = resp
    print("RESULT from {0} of chaincode from peer {1}".format(functionName, containerName))
    print(json.dumps(context.response.json(), indent = 4))
    if 'message' in resp.json():
        transactionID = context.response.json()['message']
        context.transactionID = transactionID

@then(u'I wait "{seconds}" seconds for chaincode to build')
def step_impl(context, seconds):
    """ This step takes into account the chaincodeImagesUpToDate tag, in which case the wait is reduce to some default seconds"""
    reducedWaitTime = 4
    if 'chaincodeImagesUpToDate' in context.tags:
        print("Assuming images are up to date, sleeping for {0} seconds instead of {1} in scenario {2}".format(reducedWaitTime, seconds, context.scenario.name))
        time.sleep(float(reducedWaitTime))
    else:
        time.sleep(float(seconds))

@then(u'I wait "{seconds}" seconds for transaction to be committed to block on "{containerName}"')
def step_impl(context, seconds, containerName):
    assert 'transactionID' in context, "transactionID not found in context"
    ipAddress = ipFromContainerNamePart(containerName, context.compose_containers)
    request_url = buildUrl(ipAddress, "/transactions/{0}".format(context.transactionID))
    print("GETing path = {0}".format(request_url))

    resp = requests.get(request_url, headers={'Accept': 'application/json'})
    assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
    context.response = resp

def multiRequest(context, seconds, containerDataList, pathBuilderFunc):
    """Perform a multi request against the system"""
    # Build map of "containerName" : response
    respMap = {container.containerName:None for container in containerDataList}
    # Set the max time before stopping attempts
    maxTime = datetime.now() + timedelta(seconds = int(seconds))
    for container in containerDataList:
        ipAddress = container.ipAddress
        request_url = buildUrl(ipAddress, pathBuilderFunc(context, container))

        # Loop unless failure or time exceeded
        while (datetime.now() < maxTime):
            print("GETing path = {0}".format(request_url))
            resp = requests.get(request_url, headers={'Accept': 'application/json'})
            respMap[container.containerName] = resp
        else:
            raise Exception("Max time exceeded waiting for multiRequest with current response map = {0}".format(respMap))

@then(u'I wait up to "{seconds}" seconds for transaction to be committed to all peers')
def step_impl(context, seconds):
    assert 'transactionID' in context, "transactionID not found in context"
    assert 'compose_containers' in context, "compose_containers not found in context"

    # Build map of "containerName" : resp.statusCode
    respMap = {container.containerName:0 for container in context.compose_containers}

    # Set the max time before stopping attempts
    maxTime = datetime.now() + timedelta(seconds = int(seconds))
    for container in context.compose_containers:
        ipAddress = container.ipAddress
        request_url = buildUrl(ipAddress, "/transactions/{0}".format(context.transactionID))

        # Loop unless failure or time exceeded
        while (datetime.now() < maxTime):
            print("GETing path = {0}".format(request_url))
            resp = requests.get(request_url, headers={'Accept': 'application/json'})
            if resp.status_code == 404:
                # Pause then try again
                respMap[container.containerName] = 404
                time.sleep(1)
                continue
            elif resp.status_code == 200:
                # Success, continue
                respMap[container.containerName] = 200
                break
            else:
                raise Exception("Error requesting {0}, returned result code = {1}".format(request_url, resp.status_code))
        else:
            raise Exception("Max time exceeded waiting for transactions with current response map = {0}".format(respMap))
    print("Result of request to all peers = {0}".format(respMap))
    print("")

def getContainerDataValuesFromContext(context, aliases, callback):
    """Returns the IPAddress based upon a name part of the full container name"""
    assert 'compose_containers' in context, "compose_containers not found in context"
    values = []
    containerNamePrefix = os.path.basename(os.getcwd()) + "_"
    for namePart in aliases:
        for containerData in context.compose_containers:
            if containerData.containerName.startswith(containerNamePrefix + namePart):
                values.append(callback(containerData))
                break
    return values


@then(u'I wait up to "{seconds}" seconds for transaction to be committed to peers')
def step_impl(context, seconds):
    assert 'transactionID' in context, "transactionID not found in context"
    assert 'compose_containers' in context, "compose_containers not found in context"
    assert 'table' in context, "table (of peers) not found in context"

    aliases =  context.table.headings
    containerDataList = getContainerDataValuesFromContext(context, aliases, lambda containerData: containerData)

    # Build map of "containerName" : resp.statusCode
    respMap = {container.containerName:0 for container in containerDataList}

    # Set the max time before stopping attempts
    maxTime = datetime.now() + timedelta(seconds = int(seconds))
    for container in containerDataList:
        ipAddress = container.ipAddress
        request_url = buildUrl(ipAddress, "/transactions/{0}".format(context.transactionID))

        # Loop unless failure or time exceeded
        while (datetime.now() < maxTime):
            print("GETing path = {0}".format(request_url))
            resp = requests.get(request_url, headers={'Accept': 'application/json'})
            if resp.status_code == 404:
                # Pause then try again
                respMap[container.containerName] = 404
                time.sleep(1)
                continue
            elif resp.status_code == 200:
                # Success, continue
                respMap[container.containerName] = 200
                break
            else:
                raise Exception("Error requesting {0}, returned result code = {1}".format(request_url, resp.status_code))
        else:
            raise Exception("Max time exceeded waiting for transactions with current response map = {0}".format(respMap))
    print("Result of request to all peers = {0}".format(respMap))
    print("")



@when(u'I query chaincode "{chaincodeName}" function name "{functionName}" on all peers')
def step_impl(context, chaincodeName, functionName):
    assert 'chaincodeSpec' in context, "chaincodeSpec not found in context"
    assert 'compose_containers' in context, "compose_containers not found in context"
    # Update the chaincodeSpec ctorMsg for invoke
    args = []
    if 'table' in context:
       # There is ctor arguments
       args = context.table[0].cells
    context.chaincodeSpec['ctorMsg']['function'] = functionName
    context.chaincodeSpec['ctorMsg']['args'] = args #context.table[0].cells if ('table' in context) else []
    # Invoke the POST
    chaincodeInvocationSpec = {
        "chaincodeSpec" : context.chaincodeSpec
    }
    responses = []
    for container in context.compose_containers:
        request_url = buildUrl(container.ipAddress, "/devops/{0}".format(functionName))
        print("POSTing path = {0}".format(request_url))
        resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeInvocationSpec))
        assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
        responses.append(resp)
    context.responses = responses


@when(u'I query chaincode "{chaincodeName}" function name "{functionName}" with value "{value}" on peers')
def step_impl(context, chaincodeName, functionName, value):
    assert 'chaincodeSpec' in context, "chaincodeSpec not found in context"
    assert 'compose_containers' in context, "compose_containers not found in context"
    assert 'table' in context, "table (of peers) not found in context"
    assert 'peerToSecretMessage' in context, "peerToSecretMessage map not found in context"

    aliases =  context.table.headings
    containerDataList = getContainerDataValuesFromContext(context, aliases, lambda containerData: containerData)

    # Update the chaincodeSpec ctorMsg for invoke
    context.chaincodeSpec['ctorMsg']['function'] = functionName
    context.chaincodeSpec['ctorMsg']['args'] = [value]
    # Invoke the POST
    # Make deep copy of chaincodeSpec as we will be changing the SecurityContext per call.
    chaincodeInvocationSpec = {
        "chaincodeSpec" : copy.deepcopy(context.chaincodeSpec)
    }
    responses = []
    for container in containerDataList:
        # Change the SecurityContext per call
        chaincodeInvocationSpec['chaincodeSpec']["secureContext"] = context.peerToSecretMessage[container.composeService]['enrollId']
        print("Container {0} enrollID = {1}".format(container.containerName, container.getEnv("OPENCHAIN_SECURITY_ENROLLID")))
        request_url = buildUrl(container.ipAddress, "/devops/{0}".format(functionName))
        print("POSTing path = {0}".format(request_url))
        resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeInvocationSpec), timeout=3)
        assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
        print("RESULT from {0} of chaincode from peer {1}".format(functionName, container.containerName))
        print(json.dumps(resp.json(), indent = 4))
        responses.append(resp)
    context.responses = responses




@then(u'I should get a JSON response from all peers with "{attribute}" = "{expectedValue}"')
def step_impl(context, attribute, expectedValue):
    assert 'responses' in context, "responses not found in context"
    for resp in context.responses:
        assert attribute in resp.json(), "Attribute not found in response (%s)" %(attribute)
        foundValue = resp.json()[attribute]
        assert (str(foundValue) == expectedValue), "For attribute %s, expected (%s), instead found (%s)" % (attribute, expectedValue, foundValue)

@then(u'I should get a JSON response from peers with "{attribute}" = "{expectedValue}"')
def step_impl(context, attribute, expectedValue):
    assert 'responses' in context, "responses not found in context"
    assert 'compose_containers' in context, "compose_containers not found in context"
    assert 'table' in context, "table (of peers) not found in context"

    for resp in context.responses:
        assert attribute in resp.json(), "Attribute not found in response (%s)" %(attribute)
        foundValue = resp.json()[attribute]
        assert (str(foundValue) == expectedValue), "For attribute %s, expected (%s), instead found (%s)" % (attribute, expectedValue, foundValue)

@given(u'I register with CA supplying username "{userName}" and secret "{secret}" on peers')
def step_impl(context, userName, secret):
    assert 'compose_containers' in context, "compose_containers not found in context"
    assert 'table' in context, "table (of peers) not found in context"

    # Get list of IPs to login to
    aliases =  context.table.headings
    ipAddressList = getContainerDataValuesFromContext(context, aliases, lambda containerData: containerData.ipAddress)

    secretMsg = {
        "enrollId": userName,
        "enrollSecret" : secret
    }

    # Login to each container specified
    for ipAddress in ipAddressList:
        request_url = buildUrl(ipAddress, "/registrar")
        print("POSTing path = {0}".format(request_url))

        resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(secretMsg))
        assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
        context.response = resp
        print("message = {0}".format(resp.json()))
    # Store the username in the context
    context.userName = userName
    # if we already have the chaincodeSpec, change secureContext
    if 'chaincodeSpec' in context:
        context.chaincodeSpec["secureContext"] = context.userName


@given(u'I use the following credentials for querying peers')
def step_impl(context):
    assert 'compose_containers' in context, "compose_containers not found in context"
    assert 'table' in context, "table (of peers, username, secret) not found in context"

    peerToSecretMessage = {}

    # Login to each container specified using username and secret
    for row in context.table.rows:
        peer, userName, secret = row['peer'], row['username'], row['secret']
        secretMsg = {
            "enrollId": userName,
            "enrollSecret" : secret
        }

        ipAddress = ipFromContainerNamePart(peer, context.compose_containers)
        request_url = buildUrl(ipAddress, "/registrar")
        print("POSTing to service = {0}, path = {1}".format(peer, request_url))

        resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(secretMsg))
        assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)
        context.response = resp
        print("message = {0}".format(resp.json()))
        peerToSecretMessage[peer] = secretMsg
    context.peerToSecretMessage = peerToSecretMessage


@given(u'I stop peers')
def step_impl(context):
    assert 'table' in context, "table (of peers) not found in context"
    assert 'compose_yaml' in context, "compose_yaml not found in context"
    assert 'compose_containers' in context, "compose_containers not found in context"

    services =  context.table.headings
    # Loop through services and stop them, and remove from the container data list if stopped successfully.
    for service in services:
       context.compose_output, context.compose_error, context.compose_returncode = \
           bdd_test_util.cli_call(context, ["docker-compose", "-f", context.compose_yaml, "stop", service], expect_success=True)
       assert context.compose_returncode == 0, "docker-compose failed to stop {0}".format(service)
       #remove from the containerDataList
       context.compose_containers = [containerData for  containerData in context.compose_containers if containerData.composeService != service]
    print("After stopping, the container serive list is = {0}".format([containerData.composeService for  containerData in context.compose_containers]))

@given(u'I start peers')
def step_impl(context):
    assert 'table' in context, "table (of peers) not found in context"
    assert 'compose_yaml' in context, "compose_yaml not found in context"

    services =  context.table.headings
    # Loop through services and start them
    for service in services:
       context.compose_output, context.compose_error, context.compose_returncode = \
           bdd_test_util.cli_call(context, ["docker-compose", "-f", context.compose_yaml, "start", service], expect_success=True)
       assert context.compose_returncode == 0, "docker-compose failed to start {0}".format(service)
       parseComposeOutput(context)
    print("After starting peers, the container service list is = {0}".format([containerData.composeService + ":" + containerData.ipAddress for  containerData in context.compose_containers]))

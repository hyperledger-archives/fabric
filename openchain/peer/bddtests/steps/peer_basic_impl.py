import os
import re
import time 

import sys, requests, json

import bdd_test_util

OPENCHAIN_REST_PORT = 5000

class ContainerData:
	def __init__(self, containerName, ipAddress):
		self.containerName = containerName
		self.ipAddress = ipAddress

def parseComposeOutput(context):
    """Parses the compose output results and set appropriate values into context"""
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
        print("container {0} has address = {1}".format(containerName, output.splitlines()[0]))
        containerDataList.append(ContainerData(containerName, output.splitlines()[0]))
        setattr(context, "compose_containers", containerDataList)
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
        }
    }
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
    else:
        fail('chaincodeSpec not in context')

@when(u'I invoke chaincode "{chaincodeName}" function name "{functionName}" on "{containerName}"')
def step_impl(context, chaincodeName, functionName, containerName):
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
    request_url = buildUrl(ipAddress, "/devops/invoke")
    print("POSTing path = {0}".format(request_url))

    resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeInvocationSpec))
    assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)   
    context.response = resp
    print(json.dumps(resp.json(), indent = 4))
    print("")
    #transactionID = resp.json()['message']
    #context.transactionID = transactionID

@then(u'I should have received a transactionID')
def step_impl(context):
    #TODO: Put this test back in
    #assert 'transactionID' in context, 'transactionID not found in context'
    #assert context.transactionID != ""
    pass

@when(u'I query chaincode "{chaincodeName}" function name "{functionName}" on "{containerName}"')
def step_impl(context, chaincodeName, functionName, containerName):
    invokeChaincode(context, "query", containerName)
    print(json.dumps(context.response.json(), indent = 4))
    print("")
    #raise NotImplementedError(u'STEP: When I query chaincode "example2" function name "query" with "a" on "vp0":')

def invokeChaincode(context, functionName, containerName):
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
    request_url = buildUrl(ipAddress, "/devops/{0}".format(functionName))
    print("POSTing path = {0}".format(request_url))

    resp = requests.post(request_url, headers={'Content-type': 'application/json'}, data=json.dumps(chaincodeInvocationSpec))
    assert resp.status_code == 200, "Failed to POST to %s:  %s" %(request_url, resp.text)   
    context.response = resp

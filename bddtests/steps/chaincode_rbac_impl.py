import os
import re
import time
import copy
import base64
from datetime import datetime, timedelta

import sys, requests, json

import bdd_test_util

from grpc.beta import implementations

import fabric_pb2
import chaincode_pb2
import devops_pb2

LAST_REQUESTED_TCERT="lastRequestedTCert"


@when(u'user "{enrollId}" requests a new application TCert')
def step_impl(context, enrollId):
	assert 'users' in context, "users not found in context. Did you register a user?"
	(channel, userRegistration) = bdd_test_util.getGRPCChannelAndUser(context, enrollId)
	
	stub = devops_pb2.beta_create_Devops_stub(channel)

	secret = userRegistration.getSecret()
	response = stub.EXP_GetApplicationTCert(secret,2)
	assert response.status == fabric_pb2.Response.SUCCESS, 'Failure getting TCert from {0}, for user "{1}":  {2}'.format(userRegistration.composeService,enrollId, response.msg)
	tcert = response.msg

	userRegistration.lastResult = tcert

@when(u'user "{enrollId}" stores their last result as "{tagName}"')
def step_impl(context, enrollId, tagName):
	assert 'users' in context, "users not found in context. Did you register a user?"
	# Retrieve the userRegistration from the context
	userRegistration = bdd_test_util.getUserRegistration(context, enrollId)
	userRegistration.tags[tagName] = userRegistration.lastResult
    #raise NotImplementedError(u'STEP: When user "binhn" stores his or her last result as "TCERT"')

@when(u'user "{enrollId}" sets metadata to their stored value "{tagName}"')
def step_impl(context, enrollId, tagName):
	assert 'users' in context, "users not found in context. Did you register a user?"
	# Retrieve the userRegistration from the context
	userRegistration = bdd_test_util.getUserRegistration(context, enrollId)
	assert tagName in userRegistration.tags, 'Tag "{0}" not found in user "{1}" tags'.format(tagName, enrollId)
	context.metadata = userRegistration.tags[tagName] 

@when(u'user "{enrollId}" deploys chaincode "{chaincodePath}" with ctor "{ctor}" to "{composeService}"')
def step_impl(context, enrollId, chaincodePath, ctor, composeService):
	assert 'users' in context, "users not found in context. Did you register a user?"
	(channel, userRegistration) = bdd_test_util.getGRPCChannelAndUser(context, enrollId)
	stub = devops_pb2.beta_create_Devops_stub(channel)

	args = []
	if 'table' in context:
	   # There is ctor arguments
	   args = context.table[0].cells
	ccSpec = chaincode_pb2.ChaincodeSpec(type = chaincode_pb2.ChaincodeSpec.GOLANG,
    	chaincodeID = chaincode_pb2.ChaincodeID(name="",path=chaincodePath),
    	ctorMsg = chaincode_pb2.ChaincodeInput(function = ctor, args = args))
	if 'userName' in context:
		ccSpec.secureContext = context.userName
	if 'metadata' in context:
		ccSpec.metadata = context.metadata
	ccDeploymentSpec = stub.Deploy(ccSpec, 60)
	ccSpec.chaincodeID.name = ccDeploymentSpec.chaincodeSpec.chaincodeID.name
	context.grpcChaincodeSpec = ccSpec

	#raise NotImplementedError(u'Got to here!!!')

@when(u'user "{enrollId}" gives stored value "{tagName}" to "{recipientEnrollId}"')
def step_impl(context, enrollId, tagName, recipientEnrollId):
	assert 'users' in context, "users not found in context. Did you register a user?"
	# Retrieve the userRegistration from the context
	userRegistration = bdd_test_util.getUserRegistration(context, enrollId)
	recipientUserRegistration = bdd_test_util.getUserRegistration(context, recipientEnrollId)
	# Copy value from target to recipient
	recipientUserRegistration.tags[tagName] = userRegistration.tags[tagName]


@when(u'"{enrollId}" uses application TCert "{assignerAppTCert}" to assign role "{role}" to application TCert "{assigneeAppTCert}"')
def step_impl(context, enrollId, assignerAppTCert, role, assigneeAppTCert):
	assert 'users' in context, "users not found in context. Did you register a user?"
	assert 'compose_containers' in context, "compose_containers not found in context"

	(channel, userRegistration) = bdd_test_util.getGRPCChannelAndUser(context, enrollId)

	stub = devops_pb2.beta_create_Devops_stub(channel)

	# First get binding with EXP_PrepareForTx
	secret = userRegistration.getSecret()
	response = stub.EXP_PrepareForTx(secret,2)
	assert response.status == fabric_pb2.Response.SUCCESS, 'Failure getting Binding from {0}, for user "{1}":  {2}'.format(userRegistration.composeService,enrollId, response.msg)
	binding = response.msg

	# Now produce the sigma EXP_ProduceSigma
	chaincodeInput = chaincode_pb2.ChaincodeInput(function = "addRole", args = (base64.b64encode(userRegistration.tags[assigneeAppTCert]), role) ) 
	chaincodeInputRaw = chaincodeInput.SerializeToString()
	appTCert = userRegistration.tags[assignerAppTCert]
	sigmaInput = devops_pb2.SigmaInput(secret = secret, appTCert = appTCert,  data = chaincodeInputRaw + binding)
	response = stub.EXP_ProduceSigma(sigmaInput,2)
	assert response.status == fabric_pb2.Response.SUCCESS, 'Failure prducing sigma from {0}, for user "{1}":  {2}'.format(userRegistration.composeService,enrollId, response.msg)
	sigmaOutputBytes = response.msg
	# Parse the msg bytes as a SigmaOutput message
	sigmaOutput = devops_pb2.SigmaOutput()
	sigmaOutput.ParseFromString(sigmaOutputBytes)
	print('Length of sigma = {0}'.format(len(sigmaOutput.sigma)))
	
	# Now execute the transaction with the saved binding, EXP_ExecuteWithBinding
	assert "grpcChaincodeSpec" in context, "grpcChaincodeSpec NOT found in context"
	newChaincodeSpec = chaincode_pb2.ChaincodeSpec()
	newChaincodeSpec.CopyFrom(context.grpcChaincodeSpec)
	newChaincodeSpec.metadata = sigmaOutput.asn1Encoding
	newChaincodeSpec.ctorMsg.CopyFrom(chaincodeInput)

	ccInvocationSpec = chaincode_pb2.ChaincodeInvocationSpec(chaincodeSpec = newChaincodeSpec)

	executeWithBinding = devops_pb2.ExecuteWithBinding(chaincodeInvocationSpec = ccInvocationSpec, binding = binding)

	response = stub.EXP_ExecuteWithBinding(executeWithBinding,60)
	assert response.status == fabric_pb2.Response.SUCCESS, 'Failure executeWithBinding from {0}, for user "{1}":  {2}'.format(userRegistration.composeService,enrollId, response.msg)
	context.response = response
	context.transactionID = response.msg


@then(u'"{enrollId}"\'s last transaction should have failed with message that contains "{msg}"')
def step_impl(context, enrollId, msg):
	assert 'users' in context, "users not found in context. Did you register a user?"
	assert 'compose_containers' in context, "compose_containers not found in context"
	txResult = bdd_test_util.getTxResult(context, enrollId)
	assert txResult.errorCode > 0, "Expected failure (errorCode > 0), instead found errorCode={0}".format(txResult.errorCode)
	assert msg in txResult.error, "Expected error to contain'{0}', instead found '{1}".format(msg, txResult.error)

@then(u'"{enrollId}"\'s last transaction should have succeeded')
def step_impl(context, enrollId):
	txResult = bdd_test_util.getTxResult(context, enrollId)
	assert txResult.errorCode == 0, "Expected success (errorCode == 0), instead found errorCode={0}".format(txResult.errorCode)

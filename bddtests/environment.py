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

from steps.shared.common import cli_call
from steps.shared.common import getDockerComposeFileArgsFromYamlFile

def after_scenario(context, scenario):
    get_logs = context.config.userdata.get("logs", "N")
    if get_logs.lower() == "force" or (scenario.status == "failed" and get_logs.lower() == "y"):
        print("Scenario {0} failed. Getting container logs".format(scenario.name))
        file_suffix = "_" + scenario.name.replace(" ", "_") + ".log"
        # get logs from the peer containers
        for containerData in context.compose_containers:
            with open(containerData.containerName + file_suffix, "w+") as logfile:
                sys_rc = subprocess.call(["docker", "logs", containerData.containerName], stdout=logfile, stderr=logfile)
                if sys_rc !=0 :
                    print("Cannot get logs for {0}. Docker rc = {1}".format(containerData.containerName,sys_rc))
        # get logs from the chaincode containers
        cc_output, cc_error, cc_returncode = \
            cli_call(context, ["docker",  "ps", "-f",  "name=dev-", "--format", "{{.Names}}"], expect_success=True)
        for containerName in cc_output.splitlines():
            namePart,sep,junk = containerName.rpartition("-")
            with open(namePart + file_suffix, "w+") as logfile:
                sys_rc = subprocess.call(["docker", "logs", containerName], stdout=logfile, stderr=logfile)
                if sys_rc !=0 :
                    print("Cannot get logs for {0}. Docker rc = {1}".format(namepart,sys_rc))
    if 'doNotDecompose' in scenario.tags:
        print("Not going to decompose after scenario {0}, with yaml '{1}'".format(scenario.name, context.compose_yaml))
    else:
        if 'compose_yaml' in context:
            fileArgsToDockerCompose = getDockerComposeFileArgsFromYamlFile(context.compose_yaml)

            print("Decomposing with yaml '{0}' after scenario {1}, ".format(context.compose_yaml, scenario.name))
            context.compose_output, context.compose_error, context.compose_returncode = \
                cli_call(context, ["docker-compose"] + fileArgsToDockerCompose + ["kill"], expect_success=True)
            context.compose_output, context.compose_error, context.compose_returncode = \
                cli_call(context, ["docker-compose"] + fileArgsToDockerCompose + ["rm","-f"], expect_success=True)
            # now remove any other containers (chaincodes)
            context.compose_output, context.compose_error, context.compose_returncode = \
                cli_call(context, ["docker",  "ps",  "-qa"], expect_success=True)
            if context.compose_returncode == 0:
                # Remove each container
                for containerId in context.compose_output.splitlines():
                    #print("docker rm {0}".format(containerId))
                    context.compose_output, context.compose_error, context.compose_returncode = \
                        cli_call(context, ["docker",  "rm", "-f", containerId], expect_success=True)

# stop any running peer that could get in the way before starting the tests
def before_all(context):
        cli_call(context, ["../peer/peer", "node", "stop"], expect_success=False)

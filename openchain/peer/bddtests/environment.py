
import subprocess
from steps.bdd_test_util import cli_call

def after_scenario(context, scenario):
    if scenario.status == "failed":
        get_logs = context.config.userdata.get("logs", "N")
        if get_logs.lower() == "y" :
            print("Scenario {0} failed. Getting container logs".format(scenario.name))
            file_suffix = "_" + scenario.name.replace(" ", "_") + ".log"
            for containerData in context.compose_containers:
                with open(containerData.containerName + file_suffix, "w+") as logfile:
                    sys_rc = subprocess.call(["docker", "logs", containerData.containerName], stdout=logfile, stderr=logfile)
                    if sys_rc !=0 :
                        print("Cannot get logs for {0}. Docker rc = {1}".format(containerData.containerName,sys_rc))

    if 'doNotDecompose' in scenario.tags:
        print("Not going to decompose after scenario {0}, with yaml '{1}'".format(scenario.name, context.compose_yaml))
    else:
        if 'compose_yaml' in context:
            print("Decomposing with yaml '{0}' after scenario {1}, ".format(context.compose_yaml, scenario.name))
            context.compose_output, context.compose_error, context.compose_returncode = \
                cli_call(context, ["docker-compose", "-f", context.compose_yaml, "kill"], expect_success=True)
            context.compose_output, context.compose_error, context.compose_returncode = \
                cli_call(context, ["docker-compose", "-f", context.compose_yaml, "rm","-f"], expect_success=True)
            # now remove any other containers (chaincodes)
            context.compose_output, context.compose_error, context.compose_returncode = \
                cli_call(context, ["docker",  "ps",  "-qa"], expect_success=True)
            if context.compose_returncode == 0:
                # Remove each container
                for containerId in context.compose_output.splitlines():
                    #print("docker rm {0}".format(containerId))
                    context.compose_output, context.compose_error, context.compose_returncode = \
                        cli_call(context, ["docker",  "rm", "-f", containerId], expect_success=True)

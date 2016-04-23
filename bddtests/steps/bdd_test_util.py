import os
import subprocess


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

    

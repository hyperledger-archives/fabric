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
import os.path
import re
import time
import copy
from datetime import datetime, timedelta
from behave import *

import sys, requests, json

import bdd_test_util

@then('I wait up to {waitTime} seconds for an error in the logs for peer {peerName}')
def step_impl(context, waitTime, peerName):
    pass

@then('Then ensure after {waitTime} seconds there is no errors in the logs for peer {peerName}')
def step_impl(context, waitTime, peerName):
    pass
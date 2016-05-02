#!/bin/bash

# Add any logic that is common to all (vagrant, travis, docker, etc) environments here

apt-get update -qq

# Used by CHAINTOOL
apt-get install -y default-jre

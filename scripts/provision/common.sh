#!/bin/bash

# Add any logic that is common to all (vagrant, travis, docker, etc) environments here

sudo apt-get update -qq

# Used by CHAINTOOL
sudo apt-get install -y default-jre

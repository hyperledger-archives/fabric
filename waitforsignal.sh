#!/bin/bash

pipe=/tmp/vp4start.fifo

trap "rm -f $pipe" EXIT

if [[ ! -p $pipe ]]; then
    mkfifo $pipe
fi

read < $pipe
rm -f $pipe

obc-peer peer

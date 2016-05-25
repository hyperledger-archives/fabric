#!/bin/bash

typedoc=$(which xtypedoc)
if [[ $? -ne 0 ]]; then
    echo "No typedoc found. Please install it like this:"
    echo "  npm install -g typedoc"
    echo "and rerun this shell script again."
    exit 1
fi
set -e

mkdir -p tsdoc
rm -rf tsdoc/*

typedoc -m amd \
	--name 'Hyperledger OpenBlockChain' \
	--out tsdoc \
	typedoc-special.d.ts \
	src2/crypto.ts \
	src2/hlc.ts
	

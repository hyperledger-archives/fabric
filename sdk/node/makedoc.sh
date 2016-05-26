#!/bin/bash

typedoc=$(which typedoc)
if [[ $? -ne 0 ]]; then
    echo "No typedoc found. Please install it like this:"
    echo "  npm install -g typedoc"
    echo "and rerun this shell script again."
    exit 1
fi
set -e

tv="$(typings -v)"
if [[ "${tv%.*}" < "1.0" ]]; then
    echo "You have typings ${tv} but you need 1.0 or higher."
    exit 1
fi

mkdir -p tsdoc
rm -rf tsdoc/*

typedoc -m amd \
	--name 'Hyperledger OpenBlockChain' \
	--out tsdoc \
	typedoc-special.d.ts \
	src/crypto.ts \
	src/hlc.ts

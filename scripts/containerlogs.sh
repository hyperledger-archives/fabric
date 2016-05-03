#!/bin/bash

cd $HOME/gopath/src/github.com/hyperledger/fabric/bddtests

count=$(git ls-files -o | wc -l)

git ls-files -o

echo ">>>>>>>>> CONTAINERS LOG FILES <<<<<<<<<<<<"

for (( i=1; i<"$count";i++ ))

do

file=$(echo $(git ls-files -o | sed "${i}q;d"))

echo "$file"

cat $file | curl -sT - chunk.io

done

cat testsummary.log | curl -sT - chunk.io

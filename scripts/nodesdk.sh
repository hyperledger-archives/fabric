#!/bin/bash
# Below script is to verify Node sdk and proto files
# before run NPM publish

NUM_FILES_CHANGED=$(git show --name-only $(git merge-base $ghprbActualCommit HEAD) | egrep 'sdk/node' | wc -l)
NUM_PROTO_FILES_CHANGED=$(git show --name-only $(git merge-base $ghprbActualCommit HEAD) | egrep 'protos' | wc -l)

echo "pull request commit"

git show --name-only $(git merge-base $ghprbActualCommit HEAD)

echo "Printing GIT_COMMIT number"
git show --name-only $(git merge-base $GIT_COMMIT HEAD)

echo "Number of files changed in sdk/node directory:" $NUM_FILES_CHANGED
echo "Number of Proto files changed are:" $NUM_PROTO_FILES_CHANGED



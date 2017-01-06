#!/bin/bash
# Below script is to verify Node sdk and proto files
# before run NPM publish

NUM_FILES_CHANGED=$(git diff --name-only $(git merge-base $ghprbActualCommit origin/$ghprbTargetBranch) | egrep 'sdk/node' | wc -l)
NUM_PROTO_FILES_CHANGED=$(git diff --name-only $(git merge-base $ghprbActualCommit origin/$ghprbTargetBranch) | egrep 'protos' | wc -l)
git diff --name-only $(git merge-base $ghprbActualCommit origin/$ghprbTargetBranch) | egrep 'sdk/node'
git diff --name-only $(git merge-base $ghprbActualCommit origin/$ghprbTargetBranch) | egrep 'protos'
echo "pull request commit" $ghprbActualCommit
echo "Target Branch" $ghprbTargetBranch
echo "Number of files changed in sdk/node directory:" $NUM_FILES_CHANGED
echo "Number of Proto files changed are:" $NUM_PROTO_FILES_CHANGED

if [[ "$NUM_FILES_CHANGED" -eq 0 && "$NUM_PROTO_FILES_CHANGED" -eq 0 ]]; then
echo "Node sdk files and Proto files are not changed!";
exit 0
fi
curl -L https://raw.githubusercontent.com/hyperledger/fabric/master/sdk/node/package.json -o package.json
NPM_PROD_PACKAGE_VERSION=$(cat package.json | grep version | awk -F\" '{ print $4 }')
echo "Version Number in Production is:" $NPM_PROD_PACKAGE_VERSION
cd sdk/node
NPM_PR_PACKAGE_VERSION=$(cat package.json | grep version | awk -F\" '{ print $4 }')
echo "Version number in PR is:" $NPM_PR_PACKAGE_VERSION
if test "$NPM_PROD_PACKAGE_VERSION" = "$NPM_PR_PACKAGE_VERSION";  then
echo "Package Version has to change, Please run npm version <major> <minor> <patch>"
exit 1
fi
echo "===========>>>> Every thing looks good.. Run npm publish in Merge <<< ================"

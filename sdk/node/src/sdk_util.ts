/**
 * Copyright 2016 IBM
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
/**
 * Licensed Materials - Property of IBM
 * © Copyright IBM Corp. 2016
 */

//
// Load required packages.
//

var fs = require('fs');
var grpc = require('grpc');
var uuid = require('node-uuid');

//
// Load required crypto stuff.
//

var sha3_256 = require('js-sha3').sha3_256;

//
// Load required protobufs.
//

var _timeStampProto = grpc.load(__dirname + "/protos/google/protobuf/timestamp.proto").google.protobuf.Timestamp;

//
// GenerateUUID returns an RFC4122 compliant UUID.
// http://www.ietf.org/rfc/rfc4122.txt
//

export function GenerateUUID() {
  return uuid.v4();
};

//
// GenerateTimestamp returns the current time in the google/protobuf/timestamp.proto
// structure.
//

export function GenerateTimestamp() {
  var timestamp = new _timeStampProto({ seconds: Date.now() / 1000, nanos: 0 });
  return timestamp;
}

//
// GenerateParameterHash generates a hash from the chaincode deployment parameters.
// Specifically, it hashes together the code path of the chaincode (under $GOPATH/src/),
// the initializing function name, and all of the initializing parameter values.
//

export function GenerateParameterHash(path, func, args) {
  console.log("ENTER GenerateParameterHash...");

  console.log("path = " + path);
  console.log("func = " + func);
  console.log("args = " + args);

  // Append the arguments
  var argLength = args.length;
  var argStr = "";
  for (var i = 0; i < argLength; i++) {
    argStr = argStr + args[i];
  }

  // Append the path + function + arguments
  var str = path + func + argStr;
  console.log("str ---> " + str);

  // Compute the hash
  var strHash = sha3_256(str);
  console.log("Hash of str ---> " + strHash);

  return strHash;
}

//
// GenerateDirectoryHash generates a hash of the chaincode directory contents
// and hashes that together with the chaincode parameter hash, generated above.
//

export function GenerateDirectoryHash(rootDir, chaincodeDir, hash) {
  var self = this;

  // Generate the project directory
  var projectDir = rootDir + "/" + chaincodeDir;

  // Read in the contents of the current directory
  var dirContents = fs.readdirSync(projectDir);
  var dirContentsLen = dirContents.length;

  // Go through all entries in the projet directory
  for (var i = 0; i < dirContentsLen; i++) {
    var current = projectDir + "/" + dirContents[i];

    // Check whether the entry is a file or a directory
    if (fs.statSync(current).isDirectory()) {
      // If the entry is a directory, call the function recursively.

      hash = self.GenerateDirectoryHash(rootDir, chaincodeDir + "/" + dirContents[i], hash);
    } else {
      // If the entry is a file, read it in and add the contents to the hash string

      // Read in the file as buffer
      var buf = fs.readFileSync(current);
      // Update the value to be hashed with the file content
      var toHash = buf + hash;
      // Update the value of the hash
      hash = sha3_256(toHash);
    }
  }

  return hash;
}

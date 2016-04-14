/**
 * Copyright 2015 IBM
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

var debug = require('debug')('kdf');

var sjcl = require('sjcl');
var sha3_384 = require('js-sha3').sha3_384;
var sha3_256 = require('js-sha3').sha3_256;

var crypto = require('crypto');
var aesjs = require('aes-js');


hash_sha3_256 = function (hash) {

    if (hash)
    {
        this._hash = hash._hash;
    }
    else
    {
        this.reset();
    }
};

hash_sha3_256.hash = function (data) {

    var hashBits = sjcl.codec.hex.toBits(sha3_256(bitsToBytes(data)));
    return hashBits;
};

hash_sha3_256.prototype = {

    blockSize: 1088,

    reset: function () {
        this._hash = sha3_256.create();
    },

    update: function (data) {
        this._hash.update(bitsToBytes(data));
        return this;
    },

    finalize: function () {
        var hash = this._hash.hex();
        var hashBits = sjcl.codec.hex.toBits(hash);
        this.reset();
        return hashBits;

    }
};

hash_sha3_384 = function (hash) {

    if (hash)
    {
        this._hash = hash._hash;
    }
    else
    {
        this.reset();
    }
};

hash_sha3_384.hash = function (data) {
    var hashBits = sjcl.codec.hex.toBits(sha3_384(data));

    return hashBits;
};

hash_sha3_384.prototype = {

    blockSize: 832,

    reset: function () {
        this._hash = sha3_384.create();
    },

    update: function (data) {
        this._hash.update(data);
        return this;
    },

    finalize: function () {
        var hash = this._hash.hex();
        var hashBits = sjcl.codec.hex.toBits(hash);
        //debug('finalize hashBits:\n',hashBits)
        this.reset();
        return hashBits;

    }
};
/** HKDF with the specified hash function.
 * @param {bitArray} ikm The input keying material.
 * @param {Number} keyBitLength The output key length, in bits.
 * @param {String|bitArray} salt The salt for HKDF.
 * @param {String|bitArray} info The info for HKDF.
 * @param {Object} [Hash=sjcl.hash.sha256] The hash function to use.
 * @return {bitArray} derived key.
 */
function hkdf(ikm, keyBitLength, salt, info, Hash) {
  var hmac, key, i, hashLen, loops, curOut, ret = [];

  Hash = Hash || sjcl.hash.sha256;
  if (typeof info === "string") {
    info = sjcl.codec.utf8String.toBits(info);
  }
  if (typeof salt === "string") {
    salt = sjcl.codec.utf8String.toBits(salt);
  } else if (!salt) {
    salt = [];
  }

  hmac = new sjcl.misc.hmac(salt, Hash);
  //key = hmac.mac(ikm);
  hmac.update(ikm);
  key = hmac.digest();
  debug("prk: ",bitsToBytes(key));
  hashLen = sjcl.bitArray.bitLength(key);

  loops = Math.ceil(keyBitLength / hashLen);
  if (loops > 255) {
    throw new sjcl.exception.invalid("key bit length is too large for hkdf");
  }

  hmac = new sjcl.misc.hmac(key, Hash);
  curOut = [];
  for (i = 1; i <= loops; i++) {
    hmac.update(curOut);
    hmac.update(info);
    hmac.update([sjcl.bitArray.partial(8, i)]);
    curOut = hmac.digest();
    ret = sjcl.bitArray.concat(ret, curOut);
  }
  return sjcl.bitArray.clamp(ret, keyBitLength);
};

function bitsToBytes(arr) {
    var out = [], bl = sjcl.bitArray.bitLength(arr), i, tmp;
    for (i = 0; i < bl / 8; i++) {
        if ((i & 3) === 0) {
            tmp = arr[i / 4];
        }
        out.push(tmp >>> 24);
        tmp <<= 8;
    }
    return out;
};

/** Convert from an array of bytes to a bitArray. */
function bytesToBits(bytes) {
    var out = [], i, tmp = 0;
    for (i = 0; i < bytes.length; i++) {
        tmp = tmp << 8 | bytes[i];
        if ((i & 3) === 3) {
            out.push(tmp);
            tmp = 0;
        }
    }
    if (i & 3) {
        out.push(sjcl.bitArray.partial(8 * (i & 3), tmp));
    }
    return out;
};

function hexToBytes(hex) {
    for (var bytes = [], c = 0; c < hex.length; c += 2)
        bytes.push(parseInt(hex.substr(c, 2), 16));
    return bytes;
};

function zeroBuffer(length) {
    var buf = new Buffer(length);

    buf.fill(0);

    return buf
};

exports.aesCFBDecryt = function(key,encryptedBytes){
    
    var iv = crypto.randomBytes(16);
    var aes = new aesjs.ModeOfOperation.cfb(key, iv, 16);
    
    debug("encryptedBytes: ",encryptedBytes);
    
    //need to pad encryptedBytes to multiples of 16
    var numMissingBytes = 16 - (encryptedBytes.length%16);
    debug("missingBytes: ",numMissingBytes);
    
    if (numMissingBytes > 0)
    {
        encryptedBytes = Buffer.concat([encryptedBytes,new Buffer(numMissingBytes)]);
    }
    
    debug("encryptedBytes: ",encryptedBytes);
    
    var decryptedBytes = aes.decrypt(encryptedBytes);
    
    return decryptedBytes.slice(16,decryptedBytes.length-numMissingBytes);
    
}

exports.hkdf = function(ikm, keyBitLength, salt, info, algorithm){
    
    var hash;
    var hashSize;
    
    switch (algorithm)
    {
        case "sha3-256":
            hash = hash_sha3_256;
            hashSize = 32;
            break;
        case "sha3-384":
            hash = hash_sha3_384
            hashSize = 48;
            break;
    };
    
    if (!salt)
        salt = zeroBuffer(hashSize);
        
    if (!info)
        info = "";
        
    var key = hkdf(bytesToBits(new Buffer(ikm)),keyBitLength,bytesToBits(salt),info,hash);
    
    return bitsToBytes(key);
    
}
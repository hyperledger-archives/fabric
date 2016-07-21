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

/**
 * Implement hash primitives.
 * Currently SHA3 is implemented, but needs to also add SHA2.
 *
 * NOTE: This is in pure java script to be compatible with the sjcl.hmac function.
 */
var sjcl = require('sjcl');
var jssha = require('jssha');
var sha3_256 = require('js-sha3').sha3_256;
var sha3_384 = require('js-sha3').sha3_384;


hash_sha2_256 = function (hash) {

    if (hash) {
        this._hash = hash._hash;
    }
    else {
        this.reset();
    }
};

hash_sha2_256.hash = function (data) {
    return (new sjcl.hash.sha256()).update(data).finalize();
};

hash_sha2_256.prototype = {

    blockSize: 512,

    reset: function () {
        this._hash = new sjcl.hash.sha256();
        this._hash.reset();
    },

    update: function (data) {
        this._hash.update(data);
        return this;
    },

    finalize: function () {
        var hash = this._hash.finalize();
        // var hashBits = sjcl.codec.hex.toBits(hash);
        //debug('finalize hashBits:\n',hashBits)
        this.reset();
        return hash;

    }
};


var hash_sha3_256 = function (hash) {

    if (hash) {
        this._hash = hash._hash;
    }
    else {
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

var hash_sha3_384 = function (hash) {

    if (hash) {
        this._hash = hash._hash;
    }
    else {
        this.reset();
    }
};

hash_sha3_384.hash = function (data) {
    var hashBits = sjcl.codec.hex.toBits(sha3_384(bitsToBytes(data)));
    return hashBits;
};

hash_sha3_384.prototype = {

    blockSize: 832,

    reset: function () {
        this._hash = sha3_384.create();
    },

    update: function (data) {
        this._hash.update(bitsToBytes(data));
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
}

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
}

exports.hash_sha3_256 = hash_sha3_256;
exports.hash_sha3_384 = hash_sha3_384;
exports.hash_sha2_256 = hash_sha2_256;
exports.sha2_256 = function (data) {
    return bitsToBytes(new sjcl.hash.sha256().update(bytesToBits(data)).finalize());
};

/**
 * Implement hash primitives.
 * Currently SHA3 is implemented, but needs to also add SHA2.
 *
 * NOTE: This is in pure java script to be compatible with the sjcl.hmac function.
 */
var sjcl = require('sjcl');
var jssha = require('jssha');

var hash_sha2_256 = function (hash) {

    if (hash) {
        this._hash = hash._hash;
    }
    else {
        this.reset();
    }
};

hash_sha2_256.hash = function (data) {
    var shaObj = new jssha("SHA-256", "BYTES");
    shaObj.update(bitsToBytes(data).toString());
    var hashBits = sjcl.codec.hex.toBits(shaObj.getHash("BYTES"));
    return hashBits;
};

hash_sha2_256.prototype = {

    blockSize: 1088,

    reset: function () {
        this._hash = new jssha("SHA-256","BYTES");
    },

    update: function (data) {
        this._hash.update(bitsToBytes(data));
        return this;
    },

    finalize: function () {
        var hash = this._hash.getHash('HEX');
        var hashBits = sjcl.codec.hex.toBits(hash);
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

{

    // var out = hash_sha2_256.hash(bytesToBits(new Buffer([1,2,3])));
    // console.log('%j', out)
    var shaObj = new jssha("SHA-256", "BYTES");
    shaObj.update(new Buffer([1]).toString());
    var out = shaObj.getHash("BYTES");

    console.log('%j', out)

}
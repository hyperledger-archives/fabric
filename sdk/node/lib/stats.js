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
"use strict";
/*
 * This module provides stats utilities.
 */
/**
 * The Average class keeps a rolling average based on sample values.
 * The sample weight determines how heavily to weight the most recent sample in calculating the current average.
 */
var Average = (function () {
    function Average() {
        this.setSampleWeight(0.5);
    }
    // Get the average value
    Average.prototype.getValue = function () {
        return this.avg;
    };
    /**
     * Add a sample.
     */
    Average.prototype.addSample = function (sample) {
        if (this.avg == null) {
            this.avg = sample;
        }
        else {
            this.avg = (this.avg * this.avgWeight) + (sample * this.sampleWeight);
        }
    };
    /**
     * Get the weight.
     * The weight determines how heavily to weight the most recent sample in calculating the average.
     */
    Average.prototype.getSampleWeight = function () {
        return this.sampleWeight;
    };
    /**
     * Set the weight.
     * @params weight A value between 0 and 1.
     */
    Average.prototype.setSampleWeight = function (weight) {
        if ((weight < 0) || (weight > 1)) {
            throw Error("weight must be in range [0,1]; " + weight + " is an invalid value");
        }
        this.sampleWeight = weight;
        this.avgWeight = 1 - weight;
    };
    return Average;
}());
exports.Average = Average;
/**
 * Class to keep track of an average response time.
 */
var ResponseTime = (function () {
    function ResponseTime() {
        this.avg = new Average();
    }
    ResponseTime.prototype.start = function () {
        if (this.startTime != null) {
            throw Error("started twice without stopping");
        }
        this.startTime = getCurTimeInMs();
    };
    ResponseTime.prototype.stop = function () {
        if (this.startTime == null) {
            throw Error("stopped without starting");
        }
        var elapsed = getCurTimeInMs() - this.startTime;
        this.startTime = null;
        this.avg.addSample(elapsed);
    };
    ResponseTime.prototype.cancel = function () {
        if (this.startTime == null) {
            throw Error("cancel without starting");
        }
        this.startTime = null;
    };
    // Get the average response time
    ResponseTime.prototype.getValue = function () {
        return this.avg.getValue();
    };
    return ResponseTime;
}());
exports.ResponseTime = ResponseTime;
/**
 * Calculate the rate
 */
var Rate = (function () {
    function Rate() {
        this.avg = new Average();
        this.avg.setSampleWeight(0.25);
    }
    Rate.prototype.tick = function () {
        var curTime = getCurTimeInMs();
        if (this.prevTime) {
            var elapsed = curTime - this.prevTime;
            this.avg.addSample(elapsed);
        }
        this.prevTime = curTime;
    };
    // Get the rate in ticks/ms
    Rate.prototype.getValue = function () {
        return this.avg.getValue();
    };
    return Rate;
}());
exports.Rate = Rate;
// Get the current time in milliseconds
function getCurTimeInMs() {
    return (new Date()).getTime();
}
//# sourceMappingURL=stats.js.map
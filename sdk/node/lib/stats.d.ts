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
 * The Average class keeps a rolling average based on sample values.
 * The sample weight determines how heavily to weight the most recent sample in calculating the current average.
 */
export declare class Average {
    private avg;
    private sampleWeight;
    private avgWeight;
    constructor();
    getValue(): number;
    /**
     * Add a sample.
     */
    addSample(sample: number): void;
    /**
     * Get the weight.
     * The weight determines how heavily to weight the most recent sample in calculating the average.
     */
    getSampleWeight(): number;
    /**
     * Set the weight.
     * @params weight A value between 0 and 1.
     */
    setSampleWeight(weight: number): void;
}
/**
 * Class to keep track of an average response time.
 */
export declare class ResponseTime {
    private avg;
    private startTime;
    constructor();
    start(): void;
    stop(): void;
    cancel(): void;
    getValue(): number;
}
/**
 * Calculate the rate
 */
export declare class Rate {
    private prevTime;
    private avg;
    constructor();
    tick(): void;
    getValue(): number;
}

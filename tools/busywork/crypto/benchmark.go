/*
Copyright IBM Corp. 2016. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"strconv"
)

// Call as
//
//    crypto <operation> <n>
//
// Where operation is one of
//
//    P256_sign
//    P256_verify
//    P384_sign
//    P384_verify
//
// and <n> is the number of iterations
func main() {

	operation := os.Args[1]
	n, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Printf("Can't parse %s as an integer : %s\n", os.Args[2], err)
		os.Exit(1)
	}

	var rate int

	switch operation {
	case "P256_sign":
		rate = benchmarkSignP256(n)
	case "P256_verify":
		rate = benchmarkVerifyP256(n)
	case "P384_sign":
		rate = benchmarkSignP384(n)
	case "P384_verify":
		rate = benchmarkVerifyP384(n)
	default:
		fmt.Printf("Unrecognized operation : %s\n", operation)
		os.Exit(1)
	}

	fmt.Printf("%-12s : %7d operations -> %5d operations per second\n", operation, n, rate)
}

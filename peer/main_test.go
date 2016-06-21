/*
Copyright IBM Corp. 2016 All Rights Reserved.

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

// This file is mandatory as otherwise the filebeat.test binary is not generated correctly.

import (
	"flag"
	"testing"
)

var systemTest *bool

func init() {
	systemTest = flag.Bool("systemTest", false, "Set to true when running system tests")
	// err := flag.Set("test.coverprofile", "coverage.cov")
	// if err != nil {
	// 	panic("Could not set flag")
	// }
}

// Test started when the test binary is started. Only calls main.
func TestSystem(t *testing.T) {
	//test.coverprofile=coverage.cov
	// if *systemTest {
	// 	main()
	// }
	main()
}

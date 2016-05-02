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

package shim

import (
	"testing"
)


// Test Go shim functionality that can be tested outside of a real chaincode
// context.


// TestSetLoggingLevel simply tests that the API is working, and
// case-insensitive strings are accepted. We don't actually test that the logs
// are printed at the correct level.
func TestSetLoggingLevel(t *testing.T) {
	var err error
	if err = SetLoggingLevel("debug"); err != nil {
		t.Errorf("SetLoggingLevel(debug) failed : %s", err.Error())
	}
	if err = SetLoggingLevel("INFO"); err != nil {
		t.Errorf("SetLoggingLevel(INFO) failed : %s", err.Error())
	}
	if err = SetLoggingLevel("Notice"); err != nil {
		t.Errorf("SetLoggingLevel(Notice) failed : %s", err.Error())
	}
	if err = SetLoggingLevel("WaRnInG"); err != nil {
		t.Errorf("SetLoggingLevel(WaRnInG) failed : %s", err.Error())
	}
	if err = SetLoggingLevel("ERRor"); err != nil {
		t.Errorf("SetLoggingLevel(ERRor) failed : %s", err.Error())
	}
	if err = SetLoggingLevel("critICAL"); err != nil {
		t.Errorf("SetLoggingLevel(critiCAL) failed : %s", err.Error())
	}
	if err = SetLoggingLevel("foo"); err == nil {
		t.Errorf("SetLoggingLevel(foo) should have failed but didn't!")
	}
}
	
		

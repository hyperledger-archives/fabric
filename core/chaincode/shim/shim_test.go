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

	"github.com/op/go-logging"
)

// Test Go shim functionality that can be tested outside of a real chaincode
// context.

// TestShimLogging simply tests that the APIs are working.
func TestShimLogging(t *testing.T) {
	SetLoggingLevel(LOG_CRITICAL)
	if shimLoggingLevel != LOG_CRITICAL {
		t.Errorf("shimLoggingLevel is not LOG_CRITICAL as expected")
	}
	if chaincodeLogger.IsEnabledFor(logging.DEBUG) {
		t.Errorf("The chaincodeLogger should not be enabled for DEBUG")
	}
	if !chaincodeLogger.IsEnabledFor(logging.CRITICAL) {
		t.Errorf("The chaincodeLogger should be enabled for CRITICAL")
	}
	var level LoggingLevel
	var err error
	level, err = LogLevel("debug")
	if err != nil {
		t.Errorf("LogLevel(debug) failed")
	}
	if level != LOG_DEBUG {
		t.Errorf("LogLevel(debug) did not return LOG_DEBUG")
	}
	level, err = LogLevel("INFO")
	if err != nil {
		t.Errorf("LogLevel(INFO) failed")
	}
	if level != LOG_INFO {
		t.Errorf("LogLevel(INFO) did not return LOG_INFO")
	}
	level, err = LogLevel("Notice")
	if err != nil {
		t.Errorf("LogLevel(Notice) failed")
	}
	if level != LOG_NOTICE {
		t.Errorf("LogLevel(Notice) did not return LOG_NOTICE")
	}
	level, err = LogLevel("WaRnInG")
	if err != nil {
		t.Errorf("LogLevel(WaRnInG) failed")
	}
	if level != LOG_WARNING {
		t.Errorf("LogLevel(WaRnInG) did not return LOG_WARNING")
	}
	level, err = LogLevel("ERRor")
	if err != nil {
		t.Errorf("LogLevel(ERRor) failed")
	}
	if level != LOG_ERROR {
		t.Errorf("LogLevel(ERRor) did not return LOG_ERROR")
	}
	level, err = LogLevel("critiCAL")
	if err != nil {
		t.Errorf("LogLevel(critiCAL) failed")
	}
	if level != LOG_CRITICAL {
		t.Errorf("LogLevel(critiCAL) did not return LOG_CRITICAL")
	}
	level, err = LogLevel("foo")
	if err == nil {
		t.Errorf("LogLevel(foo) did not fail")
	}
	if level != LOG_ERROR {
		t.Errorf("LogLevel(foo) did not return LOG_ERROR")
	}
}

// TestChaincodeLogging tests the logging APIs for chaincodes.
func TestChaincodeLogging(t *testing.T) {
	foo := NewLogger("foo")
	bar := NewLogger("bar")
	foo.Debugf("Foo is debugging %d", 10)
	bar.Infof("Bar in informational %d", "yes")
	foo.Noticef("NOTE NOTE NOTE")
	bar.Warningf("Danger Danger %s %s", "Will", "Robinson")
	foo.Errorf("I'm sorry Dave, I'm afraid I can't do that")
	bar.Criticalf("PI is not equal to 3.14, we computed it as %f", 4.13)
	foo.SetLevel(LOG_WARNING)
	if foo.IsEnabledFor(LOG_DEBUG) {
		t.Errorf("'foo' should not be enabled for DEBUG")
	}
	if !foo.IsEnabledFor(LOG_CRITICAL) {
		t.Errorf("'foo' should be enabled for CRITICAL")
	}
	bar.SetLevel(LOG_CRITICAL)
	if bar.IsEnabledFor(LOG_DEBUG) {
		t.Errorf("'bar' should not be enabled for DEBUG")
	}
	if !bar.IsEnabledFor(LOG_CRITICAL) {
		t.Errorf("'bar' should be enabled for CRITICAL")
	}
}

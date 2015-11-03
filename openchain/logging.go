/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package openchain

import (
	"os"

	"github.com/op/go-logging"
)

var format = logging.MustStringFormatter(
	"%{color}%{time:15:04:05.000} [%{module}] %{shortfunc} -> %{level:.4s} %{id:03x}%{color:reset} %{message}",
)

// Password is just an example type implementing the Redactor interface. Any
// time this is logged, the Redacted() function will be called.
type Password string

// Redacted for hiding the characers of password
func (p Password) Redacted() interface{} {
	return logging.Redact(string(p))
}

func init() {

	// For demo purposes, create two backend for os.Stderr.
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2 := logging.NewLogBackend(os.Stdout, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backend2Formatter := logging.NewBackendFormatter(backend2, format)

	// Only errors and more severe messages should be sent to backend1
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.ERROR, "")

	backend2Leveled := logging.AddModuleLevel(backend2Formatter)
	backend2Leveled.SetLevel(logging.DEBUG, "")

	// Set the backends to be used.
	logging.SetBackend(backend2Leveled)

	// log.Debug("debug %s", Password("secret"))
	// log.Info("info")
	// log.Notice("notice")
	// log.Warning("warning")
	// log.Error("err")
	// log.Critical("crit")

}

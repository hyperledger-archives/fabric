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

// For user-level documentation of the operation and semantics of this
// chaincode, see the README.md file contained in this directory.

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"

	busy "github.com/hyperledger/fabric/tools/busywork/busywork"
)

// counters implementation
type counters struct {
	logger        *shim.ChaincodeLogger // Our logger
	id            string                // Chaincode ID
	length        map[string]uint64     // Array lengths
	count         map[string]uint64     // Current array counts
	checkCounters bool                  // Error-check counter arrays?
	checkStatus   bool                  // Error-check the 'status' method?
}

// newCounters is a "constructor" for counters objects
func newCounters() *counters {
	c := new(counters)
	c.logger = shim.NewLogger("counters:<uninitialized>")
	c.length = map[string]uint64{}
	c.count = map[string]uint64{}
	return c
}

// debugf prints a debug message
func (c *counters) debugf(format string, args ...interface{}) {
	c.logger.Debugf(format, args...)
}

// infof prints an info message
func (c *counters) infof(format string, args ...interface{}) {
	c.logger.Infof(format, args...)
}

// errorf logs irrecoverable errors and "throws" a busywork panic
func (c *counters) errorf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	c.logger.Errorf(s)
	busy.Throw(s)
}

// criticalf logs critical irrecoverable errors and "throws" a busywork panic
func (c *counters) criticalf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	c.logger.Criticalf(s)
	busy.Throw(s)
}

// assert traps assertion failures (bugs) as panics
func (c *counters) assert(p bool, format string, args ...interface{}) {
	if !p {
		s := fmt.Sprintf("ASSERTION FAILURE:\n"+format, args...)
		fmt.Fprintf(os.Stderr, s)
		panic(s)
	}
}

// create (re-)creates one or more counter arrays and zeros their state.
func (c *counters) create(stub *shim.ChaincodeStub, args []string) (val []byte, err error) {

	// There must always be an even number of argument strings, and the odd
	// (length) strings must parse as non-0 unsigned 64-bit values.

	if (len(args) % 2) != 0 {
		c.errorf("create : Odd number of parameters; Must be pairs of ... <name> <length> ...")
	}
	name := false
	var names []string
	var lengths []uint64
	for n, x := range args {
		if name = !name; name {
			c.debugf("create : Name = %s", x)
			names = append(names, x)
		} else {
			length, err := strconv.ParseUint(x, 0, 64)
			if err != nil {
				c.errorf("create : This - '%s' - does not parse as a 64-bit integer", x)
			}
			if length <= 0 {
				c.errorf("create : Argument %d is negative or 0", n)
			}
			c.debugf("create : Length = %d", length)
			lengths = append(lengths, length)
		}
	}

	// Now create and store each array. Note that we create the arrays
	// directly as byte arrays.

	for n, name := range names {
		length := lengths[n]
		a := make([]byte, length*8)
		err := stub.PutState(name, a)
		if err != nil {
			c.criticalf("create : Error on PutState, %s[%d] : %s", name, length, err)
		}
		c.length[name] = length
		c.count[name] = 0
	}

	return
}

// incDec either increments or decrements 0 or more counter arrays. The choice
// is made based on the value of 'incr'.
func (c *counters) incDec(stub *shim.ChaincodeStub, args []string, incr int) (val []byte, err error) {

	c.assert((incr == 1) || (incr == -1), "The 'incr' parameter must be 1 or -1")

	// Check each array for existence and record the number of times it will
	// be incremented/decremented, checking for overflow/underflow along the
	// way.

	offset := map[string]uint64{}
	var names []string
	for _, name := range args {
		if _, ok := c.length[name]; !ok {
			c.errorf("incDec : Array '%s' does not exist", name)
		}
		if offset[name] == 0 {
			names = append(names, name)
		}
		offset[name]++
		if incr > 0 {
			if offset[name] > (0xffffffffffffffff - c.count[name]) {
				c.criticalf("incDec : Array '%s' would overflow", name)
			}
		} else {
			if offset[name] > c.count[name] {
				c.criticalf("incDec : Array '%s' would underflow", name)
			}
		}
	}

	// Bring the arrays into (our) memory, checking expected lengths.

	arrays := map[string][]uint64{}
	for _, name := range names {
		b, err := stub.GetState(name)
		if err != nil {
			c.criticalf("incDec : GetState() for array '%s' failed : %s", name, err)
		}
		length := c.length[name]
		if uint64(len(b)) != (length * 8) {
			c.criticalf("incDec : Array '%s' was retreived as %d bytes; Expected %d", name, len(b), length*8)
		}
		array := make([]uint64, length)
		arrays[name] = array

		c.debugf("incDec : Array '%s' initial value : %v\n", name, array)
		err = binary.Read(bytes.NewReader(b), binary.LittleEndian, &array)
		c.debugf("incDec : Array '%s' after Read    : %v\n", name, array)

		if err != nil {
			c.criticalf("incDec : Error converting bytes to uint64 : %s", err)
		}
	}

	// Increment/decrement, insuring that each array has the correct state in
	// every location. The unsigned offsets are "signed" here.

	for _, name := range names {
		count := c.count[name]
		counters := arrays[name]
		offset[name] = offset[name] * uint64(incr)
		new := count + offset[name]
		c.debugf("incDec : Array %s has count %d and offset %d", name, count, offset[name])
		for i, v := range counters {
			if c.checkCounters && (v != count) {
				c.criticalf("incDec : Element %s[%d] has value %d; Expected %d", name, i, v, count)
			}
			c.debugf("incDec : %s[%d] <- %d", name, i, new)
			counters[i] = new
		}
	}

	// Write the arrays back to the state. The new count values are only
	// recorded once this operation is successful.

	for _, name := range names {
		b := new(bytes.Buffer)
		err := binary.Write(b, binary.LittleEndian, arrays[name])
		if err != nil {
			c.criticalf("incDec : Error on binary.Write() : %s", err)
		}
		c.debugf("incDec : Putting array '%s' bytes : %v", name, b.Bytes())
		err = stub.PutState(name, b.Bytes())
		if err != nil {
			c.criticalf("incDec : Error on PutState() for array '%s' : %s", name, err)
		}
		c.count[name] += offset[name]
	}

	return
}

// initParms handles the initialization of `parms`.
func (c *counters) initParms(stub *shim.ChaincodeStub, args []string) (val []byte, err error) {

	c.infof("initParms : Command-line arguments : %v", args)

	// Define and parse the parameters
	flags := flag.NewFlagSet("initParms", flag.ContinueOnError)
	flags.StringVar(&c.id, "id", "", "A unique identifier; Allows multiple copies of this chaincode to be deployed")
	loggingLevel := flags.String("loggingLevel", "default", "The logging level of the chaincode logger. Defaults to INFO.")
	shimLoggingLevel := flags.String("shimLoggingLevel", "default", "The logging level of the chaincode 'shim' interface. Defaults to WARNING.")
	flags.BoolVar(&c.checkCounters, "checkCounters", true, "Default true; If false, no error/consistency checks are made on counter array states.")
	flags.BoolVar(&c.checkStatus, "checkStatus", true, "Default true; If false, no error/consistency checks are made in the 'status' method.")
	err = flags.Parse(args)
	if err != nil {
		c.errorf("initParms : Error during option processing : %s", err)
	}

	// Replace the original logger logging as "counters", with a new logger,
	// then set its logging level and the logging level of the shim.
	c.logger = shim.NewLogger("counters:" + c.id)
	if *loggingLevel == "default" {
		c.logger.SetLevel(shim.LogInfo)
	} else {
		level, _ := shim.LogLevel(*loggingLevel)
		c.logger.SetLevel(level)
	}
	if *shimLoggingLevel != "default" {
		level, _ := shim.LogLevel(*shimLoggingLevel)
		shim.SetLoggingLevel(level)
	}
	return
}

// queryParms handles the `parms` query
func (c *counters) queryParms(stub *shim.ChaincodeStub, args []string) (val []byte, err error) {
	flags := flag.NewFlagSet("queryParms", flag.ContinueOnError)
	flags.StringVar(&c.id, "id", "", "Uniquely identify a chaincode instance")
	err = flags.Parse(args)
	if err != nil {
		c.errorf("queryParms : Error during option processing : %s", err)
	}
	return
}

// status implements the `status` query. If the -checkStatus flag was passed
// as false, then we do not check for the array having been created, and we
// assume that the length and count obtained from the state are correct. This
// is a debug-only setting.
func (c *counters) status(stub *shim.ChaincodeStub, args []string) (val []byte, err error) {

	c.debugf("status : Entry : checkStatus = %v", c.checkStatus)

	// Run down the list of arrays, pulling their state into our memory

	arrays := map[string][]uint64{}
	for _, name := range args {
		if c.checkStatus {
			if _, ok := c.length[name]; !ok {
				c.errorf("status : Array '%s' has never been created", name)
			}
		}
		b, err := stub.GetState(name)
		if err != nil {
			c.criticalf("status : GetState() for array '%s' failed : %s", name, err)
		}
		length := len(b) / 8
		array := make([]uint64, length)
		arrays[name] = array
		err = binary.Read(bytes.NewReader(b), binary.LittleEndian, &array)
		if err != nil {
			c.criticalf("status : Error converting %d bytes of array %s to uint64 : %s", len(b), name, err)
		}
		c.debugf("status : Array %s[%d] (%d bytes)", name, length, len(b))
	}

	// Now create the result

	res := ""
	for _, name := range args {
		if res != "" {
			res += " "
		}
		actualLength := uint64(len(arrays[name]))
		actualCount := arrays[name][0]
		var expectedLength, expectedCount uint64
		if c.checkStatus {
			expectedLength = c.length[name]
			expectedCount = c.count[name]
		} else {
			expectedLength = actualLength
			expectedCount = actualCount
		}
		res += fmt.Sprintf("%d %d %d %d", expectedLength, actualLength, expectedCount, actualCount)
	}
	c.debugf("status : Final status : %s", res)
	return []byte(res), nil
}

// Init handles chaincode initialization. Only the 'parms' function is
// recognized here.
func (c *counters) Init(stub *shim.ChaincodeStub, function string, args []string) (val []byte, err error) {
	defer busy.Catch(&err)
	switch function {
	case "parms":
		return c.initParms(stub, args)
	default:
		c.errorf("Init : Function '%s' is not recognized for INIT", function)
	}
	return
}

// Invoke handles the `invoke` methods.
func (c *counters) Invoke(stub *shim.ChaincodeStub, function string, args []string) (val []byte, err error) {
	defer busy.Catch(&err)
	switch function {
	case "create":
		return c.create(stub, args)
	case "decrement":
		return c.incDec(stub, args, -1)
	case "increment":
		return c.incDec(stub, args, 1)
	default:
		c.errorf("Invoke : Function '%s' is not recognized for INVOKE", function)
	}
	return
}

// Query handles the `query` methods.
func (c *counters) Query(stub *shim.ChaincodeStub, function string, args []string) (val []byte, err error) {
	defer busy.Catch(&err)
	switch function {
	case "parms":
		return c.queryParms(stub, args)
	case "status":
		return c.status(stub, args)
	default:
		c.errorf("Query : Function '%s' is not recognized for QUERY", function)
	}
	return
}

func main() {

	c := newCounters()

	c.logger.SetLevel(shim.LogInfo)
	shim.SetLoggingLevel(shim.LogWarning)

	// This is required because we need the lengths (len()) of our arrays to
	// support large (> 2^32) byte arrays.
	c.assert(busy.SizeOfInt() == 8, "The 'counters' chaincode is only supported on platforms where Go integers are 8-byte integers")

	err := shim.Start(c)
	if err != nil {
		c.logger.Criticalf("main : Error starting counters chaincode: %s", err)
		os.Exit(1)
	}
}

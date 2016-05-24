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

package perfutil

import (
	"fmt"
	"time"
)

var (
	enableTrace bool
	perfUuid string
	allTransactionTraces map[string] *traceOperation
)

type traceOperation struct {
	name string
	start int64
	end int64
	succ bool
	next *traceOperation
}

// set Trace uuid
func SetPerfUuid(uuid string)  {
	perfUuid = uuid
	return
}

// get Trace uuid
func GetPerfUuid() (string) {
	return perfUuid
}

//Trace state
func TraceState() (bool) {
	return enableTrace
}

//enable/disable trace
func SetTrace(val bool) {
	enableTrace = val
	if val {
		allTransactionTraces = make(map[string] *traceOperation)
	} else {
		allTransactionTraces = nil
	}
}

//get the pointer to the end of the link list
func GetPT(uuid string) *traceOperation {
	if enableTrace {
		to := allTransactionTraces[uuid]
		if to == nil {
			fmt.Printf("[GetPT] Transaction trace nil. \n")
			return to
		}
		for {
			if to.next == nil {
				break
			}
			to = to.next
		}
		return to
	}
	return nil
}

//create the hash table entry and the first trace
func CreatePTHdr(uuid string, name string, now int64) {
	if enableTrace {
		perfUuid = uuid
		if allTransactionTraces[uuid] != nil {
			fmt.Printf("Transaction trace exists\n")
			return
		}

		if now == 0 {
			now = time.Now().UnixNano()
		}
		allTransactionTraces[uuid] = &traceOperation{ name: name, start: now }
	}
}

//create trace from a divide (gRPC, channel, etc)
func CreatePTOPFromCC(tt *traceOperation, name string, since int64) *traceOperation {
	if tt == nil {
		return nil
	}

	to := CreatePTOP(tt, name, since)
	//Message (e.g., ChaincodeMessage) just computes time taken across a divide (grpc, channel, etc)
	//for this, if we got the message, it is a success
	EndCurrPTOP(to, true)
	return to
}

//create trace
func CreatePTOP(tt *traceOperation, name string, now int64) *traceOperation {
	if tt == nil {
		fmt.Printf("[CreatePTOP] ... Transaction trace nil: invalid\n")
		return nil
	}
	if name == "" {
		panic("blank PTOP name")
	}

	if now == 0 {
		now = time.Now().UnixNano()
	}

	// sanity check current entry
	if tt.start == 0 || tt.end == 0 {
		fmt.Printf("ERROR create called for [%s] when current [%s] not set correctly (%d, %d)\n", name, tt.name, tt.start, tt.end)
	}

	tt.next =  &traceOperation{ name: name, start: now }
	return tt.next
}

// end the current trace
func EndCurrPTOP(tt *traceOperation, res bool) {
	if tt == nil {
		fmt.Printf("ERROR endCurrent called when create not called\n")
		return
	}

	if tt.start == 0 {
		fmt.Printf("ERROR endCurrent called when start not set for [%s]\n", tt.name)
		return
	}
	tt.end = time.Now().UnixNano()
	tt.succ = res
}

// PerfTraceHandler handles performance trace instrumentation request
// input:
//    uuid: hash table id
//    name: name of this entry
//    now:  timestamp
//    res:  result of the corresponding API
//    action: type of actions
//            Create - create hash table entry and start a link list entry with the first entry having the name provided
//			do not care: res
//            CreatePTOP - close current link list entry and create a new link list entry with the name provided
//			do not care: res
//            EndPTOP - close current link list entry and end current link list
//			do not care: name and now
//            CreatePTOPFrom - close current link list entry, create a link list entry with the name and timestamps (now) provided,
//                             and start a new one from Now() with the same trace name as the newly created entry with preface "Since_"
//			do not care: res
func PerfTraceHandler(uuid string, name string, now int64, res bool, action string) {
	if !enableTrace {
		return 
	}

	// sanity check uuid
	if action != "Create" {
		if perfUuid == "" {
			fmt.Printf("perf trace uuid has not been set yet. name[%s]\n", name)
			return
		}
		if perfUuid != uuid {
			fmt.Printf("perf trace uuid [%s] does not match with perfUuid [%d].\n", uuid, perfUuid)
			return
		}
	}


	switch action {
	case "Create":
		CreatePTHdr(uuid, name, now)

	case "CreatePTOP":
		tt := GetPT(uuid)
		EndCurrPTOP(tt, true)
		tt = GetPT(uuid)
		CreatePTOP(tt, name, now)
	
	case "EndPTOP":
		tt := GetPT(uuid)
		EndCurrPTOP(tt, res)
		fmt.Printf("[PerfTraceHandler] ... action[%s]\n", action)
 
	case "CreatePTOPFrom":
		tt := GetPT(uuid)
		EndCurrPTOP(tt, res)
		tt = GetPT(uuid)
		CreatePTOPFromCC(tt, name, now)
		tt = GetPT(uuid)
		s_name := "Since_"+name
		CreatePTOP(tt, s_name, now)

	default:
		fmt.Printf("[PerfTraceHandler]: action [%s] invalid.\n", action)

	}

	return 
}

func DumpTx(uuid string, tt *traceOperation){
	if tt == nil {
		fmt.Printf("[DumpTx]trace pointer invalid [nil]\n")
		return
	}

	top := tt

	var total int64

	fmt.Printf("Stats for tx %s\n", uuid)
	for {
		if top == nil {
			break
		}
		et := top.end-top.start
		fmt.Printf("\t%s %f ms\n", top.name, float64(et)/1000000)
		total += et
		top = top.next
	}
	fmt.Printf("\n\ttotal %f ms\n", float64(total)/1000000)
}

func DumpStats() {
	if !enableTrace {
		fmt.Printf("[DumpStats]performance trace instrumentation not enabled\n")
		return
	}
	for uuid := range allTransactionTraces {
		tt := allTransactionTraces[uuid]
		DumpTx(uuid, tt)
	}
}


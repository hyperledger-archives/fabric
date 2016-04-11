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

package perfstat

import (
	"bytes"
	"fmt"
	"github.com/op/go-logging"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const ENABLE_STATS = false
const PRINT_PERIODICALLY = true
const PRINT_INTERVAL = 10000 //Millisecond
const COMMON_PREFIX = "github.com/hyperledger/fabric/core/ledger"
const COMMON_PREFIX_LEN = len(COMMON_PREFIX)

var holder *statsHolder
var once sync.Once
var logger = logging.MustGetLogger("ledger.perfstat")

type statsHolder struct {
	rwLock sync.RWMutex
	m      map[string]*stat
}

func init() {
	if !ENABLE_STATS {
		return
	}
	holder = &statsHolder{m: make(map[string]*stat)}
	if PRINT_PERIODICALLY {
		go printStatsPeriodically()
	}
}

func UpdateTimeStat(id string, startTime time.Time) {
	if !ENABLE_STATS {
		return
	}
	path := getCallerInfo()
	statName := fmt.Sprintf("%s:%s", path, id)
	stat := getOrCreateStat(statName, "", 0)
	stat.updateDataStat(time.Since(startTime).Nanoseconds())
}

func UpdateDataStat(id string, value int64) {
	if !ENABLE_STATS {
		return
	}
	path := getCallerInfo()
	statName := fmt.Sprintf("%s:%s", path, id)
	stat := getOrCreateStat(statName, "", 0)
	stat.updateDataStat(value)
}

func ResetStats() {
	if !ENABLE_STATS {
		return
	}
	holder.rwLock.Lock()
	defer holder.rwLock.Unlock()
	for _, v := range holder.m {
		v.reset()
	}
}

func getOrCreateStat(name string, file string, line int) *stat {
	holder.rwLock.RLock()
	stat, ok := holder.m[name]
	if ok {
		holder.rwLock.RUnlock()
		return stat
	}

	holder.rwLock.RUnlock()
	holder.rwLock.Lock()
	defer holder.rwLock.Unlock()
	stat, ok = holder.m[name]
	if !ok {
		stat = newStat(name, fmt.Sprintf("%s:%d", file, line))
		holder.m[name] = stat
	}
	return stat
}

func printStatsPeriodically() {
	for {
		PrintStats()
		time.Sleep(time.Duration(int64(PRINT_INTERVAL) * time.Millisecond.Nanoseconds()))
	}
}

func PrintStats() {
	if !ENABLE_STATS {
		return
	}
	holder.rwLock.RLock()
	defer holder.rwLock.RUnlock()
	logger.Info("Stats.......Start")
	var paths []string
	for k, _ := range holder.m {
		paths = append(paths, k)
	}
	sort.Strings(paths)
	for _, k := range paths {
		v := holder.m[k]
		logger.Info(v.String())
	}
	logger.Info("Stats.......Finish")
}

func getCallerInfo() string {
	pc := make([]uintptr, 10)
	runtime.Callers(3, pc)
	var path bytes.Buffer
	j := 0
	for i := range pc {
		f := runtime.FuncForPC(pc[i])
		funcName := f.Name()
		if strings.HasPrefix(funcName, COMMON_PREFIX) {
			j = i
		} else {
			break
		}
	}

	for i := j; i >= 0; i-- {
		f := runtime.FuncForPC(pc[i])
		funcName := f.Name()
		funcNameShort := funcName[COMMON_PREFIX_LEN:]
		path.WriteString(funcNameShort)
		if i > 0 {
			path.WriteString(" -> ")
		}
	}

	return path.String()
}

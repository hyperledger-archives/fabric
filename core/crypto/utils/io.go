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

package utils

import (
	"encoding/base64"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// DirMissingOrEmpty checks is a directory is missin or empty
func DirMissingOrEmpty(path string) (bool, error) {
	dirExists, err := DirExists(path)
	if err != nil {
		return false, err
	}
	if !dirExists {
		return true, nil
	}

	dirEmpty, err := DirEmpty(path)
	if err != nil {
		return false, err
	}
	if dirEmpty {
		return true, nil
	}
	return false, nil
}

// DirExists checks if a directory exists
func DirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// DirEmpty checks if a directory is empty
func DirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// FileMissing checks if a file is missing
func FileMissing(path string, name string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, name))
	if err != nil {
		return true, err
	}
	return false, nil
}

// FilePathMissing returns true if the path is missing, false otherwise.
func FilePathMissing(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		return true, err
	}
	return false, nil
}

// DecodeBase64 decodes from Base64
func DecodeBase64(in string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(in)
}

// EncodeBase64 encodes to Base64
func EncodeBase64(in []byte) string {
	return base64.StdEncoding.EncodeToString(in)
}

// IntArrayEquals checks if the arrays of ints are the same
func IntArrayEquals(a []int, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// IsTCPPortOpen checks the tcp port is open
func IsTCPPortOpen(laddr string) error {
	lis, err := net.Listen("tcp", laddr)
	if err != nil {
		return err
	}
	lis.Close()
	return nil
}

var seed uint32
var randmu sync.Mutex

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextSuffix() string {
	randmu.Lock()
	r := seed
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	seed = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

// TempFile returns a temporary file path with the respect the system temp dir.
func TempFile(dir, prefix string) (name string, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	nconflict := 0
	for i := 0; i < 10000; i++ {
		name = filepath.Join(dir, prefix+nextSuffix())
		f, err := os.Stat(name)
		if f != nil || os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				randmu.Lock()
				seed = reseed()
				randmu.Unlock()
			}
			continue
		}
		break
	}
	return
}

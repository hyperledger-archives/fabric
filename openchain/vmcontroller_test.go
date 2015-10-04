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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

/**** not using actual files from file system for testing.... use these funcs if we want to do that
func getCodeChainBytes(pathtocodechain string) (io.Reader, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := writeCodeChainTar(pathtocodechain, tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting codechain tar: %s", err))
	}
        ioutil.WriteFile("/tmp/chaincode.tar", inputbuf.Bytes(), 0644)
	return inputbuf, nil
}

func writeCodeChainTar(pathtocodechain string, tw *tar.Writer) error {
	root_directory := pathtocodechain //use full path
	fmt.Printf("tar %s start(%s)\n", root_directory, time.Now())

	walkFn := func(path string, info os.FileInfo, err error) error {
	        fmt.Printf("path %s(%s)\n", path, info.Name())
                if info == nil {
	             return errors.New(fmt.Sprintf("Error walking the path: %s", path))
                }

		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external root_directory variable
		//new_path := fmt.Sprintf("%s", path[len(root_directory):])
		new_path := info.Name()

		if len(new_path) == 0 {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		if h, err := tar.FileInfoHeader(info, new_path); err != nil {
			fmt.Printf(fmt.Sprintf("Error getting FileInfoHeader: %s\n", err))
			return err
		} else {
			h.Name = new_path
			if err = tw.WriteHeader(h); err != nil {
				fmt.Printf(fmt.Sprintf("Error writing header: %s\n", err))
				return err
			}
		}
		if length, err := io.Copy(tw, fr); err != nil {
			return err
		} else {
			fmt.Printf("Length of entry = %d\n", length)
		}
		return nil
	}

	if err := filepath.Walk(root_directory, walkFn); err != nil {
		fmt.Printf("Error walking root_directory: %s\n", err)
		return err
	} else {
		// Write the tar file out
		if err := tw.Close(); err != nil {
                    return err
		}
	}
	fmt.Printf("tar end = %s\n", time.Now())
	return nil
}
*********************/

func getCodeChainBytesInMem() (io.Reader, error) {
	startTime := time.Now()
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	dockerFileContents := []byte("FROM busybox:latest\n\nCMD echo \"hello obc\"\n")
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tr.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tr.Write([]byte(dockerFileContents))
	tr.Close()
	gw.Close()
	ioutil.WriteFile("/tmp/chaincode.tar", inputbuf.Bytes(), 0644)
	return inputbuf, nil
}

func TestVMCBuildImage(t *testing.T) {
	viper.SetEnvPrefix("openchain")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigName("vm")                    // name of config file (without extension)
	viper.AddConfigPath("./")                    // path to look for the config file in
	if err := viper.ReadInConfig(); err != nil { // Find and read the config file
		t.Fail()
		t.Logf("Error reading config file: %s", err)
		return
	}

	//this creates a singleton... Peer will call it once to initialize
	vmc := NewVMController()

	var ctxt context.Context = context.Background()

	//get the tarball for codechain
	tarRdr, err := getCodeChainBytesInMem()
	if err != nil {
		t.Fail()
		t.Logf("Error reading tar file: %s", err)
		return
	}

	c := make(chan struct{})

	//creat a CreateImageReq obj and send it to VMController.Process
	go func() {
		defer close(c)
		args := make([]string, 1)
		args[0] = "-i"
		cir := CreateImageReq{id: "simple", reader: tarRdr, args: args}
		_, err := vmc.Process(ctxt, "Docker", cir)
		if err != nil {
			t.Fail()
			t.Logf("Error creating image: %s", err)
			return
		}
	}()

	//wait for VMController to complete.
	<-c
}

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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
	"github.com/spf13/viper"

	pb "github.com/openblockchain/obc-peer/protos"
)

type VM struct {
	Client *docker.Client
}

func NewVM() (*VM, error) {
	endpoint := viper.GetString("vm.endpoint")
	log.Info("Creating VM with endpoint: %s", endpoint)
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	VM := &VM{Client: client}
	return VM, nil
}

var vmLogger = logging.MustGetLogger("peer")

func (vm *VM) ListImages(context context.Context) error {
	imgs, err := vm.Client.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		return err
	}
	for _, img := range imgs {
		fmt.Println("ID: ", img.ID)
		fmt.Println("RepoTags: ", img.RepoTags)
		fmt.Println("Created: ", img.Created)
		fmt.Println("Size: ", img.Size)
		fmt.Println("VirtualSize: ", img.VirtualSize)
		fmt.Println("ParentId: ", img.ParentID)
	}

	return nil
}

func buildVMName(spec *pb.ChainletSpec) string {
	vmName := fmt.Sprintf("%s_%s", viper.GetString("peer.peerId"), strings.Replace(spec.ChainletID.Url, string(os.PathSeparator), ".", -1))
	vmLogger.Debug("return VM name: %s", vmName)
	return vmName
}

func (vm *VM) BuildChaincodeContainer(spec *pb.ChainletSpec) error {
	//inputbuf, err := vm.GetPeerPackageBytes()
	inputbuf, err := vm.GetChaincodePackageBytes(spec)

	if err != nil {
		return errors.New(fmt.Sprintf("Error building Peer container: %s", err))
	}
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         buildVMName(spec),
		Pull:         true,
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		return errors.New(fmt.Sprintf("Error building Chaincode container: %s", err))
	}
	return nil
}

func (vm *VM) BuildPeerContainer() error {
	//inputbuf, err := vm.GetPeerPackageBytes()
	inputbuf, err := vm.getPackageBytes(vm.writePeerPackage)

	if err != nil {
		return errors.New(fmt.Sprintf("Error building Peer container: %s", err))
	}
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         "openchain-peer",
		Pull:         true,
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		return errors.New(fmt.Sprintf("Error building Peer container: %s", err))
	}
	return nil
}

func (vm *VM) GetChaincodePackageBytes(spec *pb.ChainletSpec) (io.Reader, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := vm.writeChaincodePackage(spec, tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting Peer package: %s", err))
	}
	return inputbuf, nil
}

func (vm *VM) GetPeerPackageBytes() (io.Reader, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := vm.writePeerPackage(tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting Peer package: %s", err))
	}
	return inputbuf, nil
}

//type tarWriter func()

func (vm *VM) getPackageBytes(writerFunc func(*tar.Writer) error) (io.Reader, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := writerFunc(tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting package bytes: %s", err))
	}
	return inputbuf, nil
}

func (vm *VM) writeChaincodePackage(spec *pb.ChainletSpec, tw *tar.Writer) error {
	startTime := time.Now()

	// Dynamically create the Dockerfile from the base config and required additions
	newRunLine := fmt.Sprintf("RUN go install %s && cp src/github.com/openblockchain/obc-peer/openchain.yaml $GOPATH/bin", spec.ChainletID.Url)
	dockerFileContents := fmt.Sprintf("%s\n%s", viper.GetString("chainlet.golang.Dockerfile"), newRunLine)
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tw.Write([]byte(dockerFileContents))
	err := vm.writeGopathSrc(tw)
	if err != nil {
		return errors.New(fmt.Sprintf("Error writing Chaincode package contents: %s", err))
	}
	return nil
}

func (vm *VM) writePeerPackage(tw *tar.Writer) error {
	startTime := time.Now()

	dockerFileContents := viper.GetString("peer.Dockerfile")
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tw.Write([]byte(dockerFileContents))
	err := vm.writeGopathSrc(tw)
	if err != nil {
		return errors.New(fmt.Sprintf("Error writing Peer package contents: %s", err))
	}
	return nil
}

func (vm *VM) writeGopathSrc(tw *tar.Writer) error {
	root_directory := fmt.Sprintf("%s%s%s", os.Getenv("GOPATH"), string(os.PathSeparator), "src")
	vmLogger.Info("root_directory = %s", root_directory)
	//inputbuf := bytes.NewBuffer(nil)
	//tw := tar.NewWriter(inputbuf)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external root_directory variable
		new_path := fmt.Sprintf("src%s", path[len(root_directory):])
		//new_path := path[len(root_directory):]
		if len(new_path) == 0 {
			return nil
		}

		// If path includes .git, ignore
		if strings.Contains(path, ".git") {
			return nil
		}

		// Allow only go files and yaml
		if !(strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".yaml")) {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		if h, err := tar.FileInfoHeader(info, new_path); err != nil {
			vmLogger.Error(fmt.Sprintf("Error getting FileInfoHeader: %s", err))
			return err
		} else {
			h.Name = new_path
			if err = tw.WriteHeader(h); err != nil {
				vmLogger.Error(fmt.Sprintf("Error writing header: %s", err))
				return err
			}
		}
		if _, err := io.Copy(tw, fr); err != nil {
			return err
		} else {
			//vmLogger.Debug("Length of entry = %d", length)
		}
		return nil
	}

	if err := filepath.Walk(root_directory, walkFn); err != nil {
		vmLogger.Info("Error walking root_directory: %s", err)
		return err
	} else {
		// Write the tar file out
		if err := tw.Close(); err != nil {
			return err
		}
		//ioutil.WriteFile("/tmp/chainlet_deployment.tar", inputbuf.Bytes(), 0644)
	}
	return nil
}

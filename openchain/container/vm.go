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

package container

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/spf13/viper"

	"github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"
)

// VM implemenation of VM management functionality.
type VM struct {
	Client *docker.Client
}

// NewVM creates a new VM instance.
func NewVM() (*VM, error) {
	endpoint := viper.GetString("vm.endpoint")
	vmLogger.Info("Creating VM with endpoint: %s", endpoint)
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	VM := &VM{Client: client}
	return VM, nil
}

var vmLogger = logging.MustGetLogger("container")

// ListImages list the images available
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

// BuildChaincodeContainer builds the container for the supplied chaincode specification
func (vm *VM) BuildChaincodeContainer(spec *pb.ChaincodeSpec) ([]byte, error) {
	chaincodePkgBytes, err := vm.GetChaincodePackageBytes(spec)
	if err != nil {
		return nil, fmt.Errorf("Error building Chaincode container: %s", err)
	}
	err = vm.buildChaincodeContainerUsingDockerfilePackageBytes(spec, bytes.NewReader(chaincodePkgBytes))
	if err != nil {
		return nil, fmt.Errorf("Error building Chaincode container: %s", err)
	}
	return chaincodePkgBytes, nil
}

// Builds the Chaincode image using the supplied Dockerfile package contents
func (vm *VM) buildChaincodeContainerUsingDockerfilePackageBytes(spec *pb.ChaincodeSpec, inputbuf io.Reader) error {
	outputbuf := bytes.NewBuffer(nil)
	vmName, err := GetVMName(spec.ChaincodeID)
	if err != nil {
		return fmt.Errorf("Error building chaincode using package bytes: %s", err)
	}
	opts := docker.BuildImageOptions{
		Name:         vmName,
		Pull:         true,
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		return fmt.Errorf("Error building Chaincode container: %s", err)
	}
	return nil
}

// BuildPeerContainer builds the image for the Peer to be used in development network
func (vm *VM) BuildPeerContainer() error {
	//inputbuf, err := vm.GetPeerPackageBytes()
	inputbuf, err := vm.getPackageBytes(vm.writePeerPackage)

	if err != nil {
		return fmt.Errorf("Error building Peer container: %s", err)
	}
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         "openchain-peer",
		Pull:         true,
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		return fmt.Errorf("Error building Peer container: %s", err)
	}
	return nil
}

// GetChaincodePackageBytes returns the gzipped tar image used for docker build of supplied Chaincode package
func (vm *VM) GetChaincodePackageBytes(spec *pb.ChaincodeSpec) ([]byte, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := vm.writeChaincodePackage(spec, tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, fmt.Errorf("Error getting Peer package: %s", err)
	}
	return inputbuf.Bytes(), nil
}

// GetPeerPackageBytes returns the gzipped tar image used for docker build of Peer
func (vm *VM) GetPeerPackageBytes() (io.Reader, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := vm.writePeerPackage(tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, fmt.Errorf("Error getting Peer package: %s", err)
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
		return nil, fmt.Errorf("Error getting package bytes: %s", err)
	}
	return inputbuf, nil
}

func (vm *VM) writeChaincodePackage(spec *pb.ChaincodeSpec, tw *tar.Writer) error {
	startTime := time.Now()

	// Dynamically create the Dockerfile from the base config and required additions
	var newRunLine string
	if strings.HasPrefix(spec.ChaincodeID.Url, "http://") {
		urlLocation := spec.ChaincodeID.Url[7:]
		newRunLine = fmt.Sprintf("RUN go get %s && cp src/github.com/openblockchain/obc-peer/openchain.yaml $GOPATH/bin", urlLocation)
	} else if strings.HasPrefix(spec.ChaincodeID.Url, "https://") {
		urlLocation := spec.ChaincodeID.Url[8:]
		newRunLine = fmt.Sprintf("RUN go get %s && cp src/github.com/openblockchain/obc-peer/openchain.yaml $GOPATH/bin", urlLocation)
	} else {
		newRunLine = fmt.Sprintf("RUN go install %s && cp src/github.com/openblockchain/obc-peer/openchain.yaml $GOPATH/bin", spec.ChaincodeID.Url)
	}
	dockerFileContents := fmt.Sprintf("%s\n%s", viper.GetString("chaincode.golang.Dockerfile"), newRunLine)
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tw.Write([]byte(dockerFileContents))
	err := vm.writeGopathSrc(tw)
	if err != nil {
		return fmt.Errorf("Error writing Chaincode package contents: %s", err)
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
		return fmt.Errorf("Error writing Peer package contents: %s", err)
	}
	return nil
}

func (vm *VM) writeGopathSrc(tw *tar.Writer) error {
	rootDirectory := fmt.Sprintf("%s%s%s", os.Getenv("GOPATH"), string(os.PathSeparator), "src")
	vmLogger.Info("rootDirectory = %s", rootDirectory)
	//inputbuf := bytes.NewBuffer(nil)
	//tw := tar.NewWriter(inputbuf)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external rootDirectory variable
		newPath := fmt.Sprintf("src%s", path[len(rootDirectory):])
		//newPath := path[len(rootDirectory):]
		if len(newPath) == 0 {
			return nil
		}

		// If path includes .git, ignore
		if strings.Contains(path, ".git") {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		h, err := tar.FileInfoHeader(info, newPath)
		if err != nil {
			vmLogger.Error(fmt.Sprintf("Error getting FileInfoHeader: %s", err))
			return err
		}
		h.Name = newPath
		if err = tw.WriteHeader(h); err != nil {
			vmLogger.Error(fmt.Sprintf("Error writing header: %s", err))
			return err
		}
		if _, err := io.Copy(tw, fr); err != nil {
			return err
		}
		return nil
	}

	if err := filepath.Walk(rootDirectory, walkFn); err != nil {
		vmLogger.Info("Error walking rootDirectory: %s", err)
		return err
	}
	// Write the tar file out
	if err := tw.Close(); err != nil {
		return err
	}
	//ioutil.WriteFile("/tmp/chaincode_deployment.tar", inputbuf.Bytes(), 0644)
	return nil
}

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
	"time"

	"golang.org/x/net/context"

	"github.com/spf13/viper"

	"github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/chaincode/platforms"
	cutil "github.com/openblockchain/obc-peer/openchain/container/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

func newDockerClient() (client *docker.Client, err error) {
	//QQ: is this ok using config properties here so deep ? ie, should we read these in main and stow them away ?
	endpoint := viper.GetString("vm.endpoint")
	vmLogger.Info("Creating VM with endpoint: %s", endpoint)
	if viper.GetBool("vm.docker.tls.enabled") {
		cert := viper.GetString("vm.docker.tls.cert.file")
		key := viper.GetString("vm.docker.tls.key.file")
		ca := viper.GetString("vm.docker.tls.ca.file")
		client, err = docker.NewTLSClient(endpoint, cert, key, ca)
	} else {
		client, err = docker.NewClient(endpoint)
	}
	return
}

// VM implemenation of VM management functionality.
type VM struct {
	Client *docker.Client
}

// NewVM creates a new VM instance.
func NewVM() (*VM, error) {
	client, err := newDockerClient()
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
	chaincodePkgBytes, err := GetChaincodePackageBytes(spec)
	if err != nil {
		return nil, fmt.Errorf("Error getting chaincode package bytes: %s", err)
	}
	err = vm.buildChaincodeContainerUsingDockerfilePackageBytes(spec, chaincodePkgBytes)
	if err != nil {
		return nil, fmt.Errorf("Error building Chaincode container: %s", err)
	}
	return chaincodePkgBytes, nil
}

// GetChaincodePackageBytes creates bytes for docker container generation using the supplied chaincode specification
func GetChaincodePackageBytes(spec *pb.ChaincodeSpec) ([]byte, error) {
	if spec == nil || spec.ChaincodeID == nil {
		return nil, fmt.Errorf("invalid chaincode spec")
	}
	if spec.ChaincodeID.Name != "" {
		return nil, fmt.Errorf("chaincode name exists")
	}

	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tw := tar.NewWriter(gw)

	platform, err := platforms.Find(spec.Type)
	if err != nil {
		return nil, err
	}

	err = platform.WritePackage(spec, tw)
	if err != nil {
		return nil, err
	}

	tw.Close()
	gw.Close()

	if err != nil {
		return nil, err
	}

	chaincodePkgBytes := inputbuf.Bytes()

	return chaincodePkgBytes, nil
}

// Builds the Chaincode image using the supplied Dockerfile package contents
func (vm *VM) buildChaincodeContainerUsingDockerfilePackageBytes(spec *pb.ChaincodeSpec, code []byte) error {
	outputbuf := bytes.NewBuffer(nil)
	vmName := spec.ChaincodeID.Name
	inputbuf := bytes.NewReader(code)
	opts := docker.BuildImageOptions{
		Name:         vmName,
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		vmLogger.Debug(fmt.Sprintf("Failed Chaincode docker build:\n%s\n", outputbuf.String()))
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
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		vmLogger.Debug(fmt.Sprintf("Failed Peer docker build:\n%s\n", outputbuf.String()))
		return fmt.Errorf("Error building Peer container: %s\n", err)
	}
	return nil
}

// BuildObccaContainer builds the image for the obcca to be used in development network
func (vm *VM) BuildObccaContainer() error {
	inputbuf, err := vm.getPackageBytes(vm.writeObccaPackage)

	if err != nil {
		return fmt.Errorf("Error building obcca container: %s", err)
	}
	outputbuf := bytes.NewBuffer(nil)
	opts := docker.BuildImageOptions{
		Name:         "obcca",
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := vm.Client.BuildImage(opts); err != nil {
		vmLogger.Debug(fmt.Sprintf("Failed obcca docker build:\n%s\n", outputbuf.String()))
		return fmt.Errorf("Error building obcca container: %s\n", err)
	}
	return nil
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

func (vm *VM) writePeerPackage(tw *tar.Writer) error {
	startTime := time.Now()

	dockerFileContents := viper.GetString("peer.Dockerfile")
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tw.Write([]byte(dockerFileContents))
	err := cutil.WriteGopathSrc(tw, "")
	if err != nil {
		return fmt.Errorf("Error writing Peer package contents: %s", err)
	}
	return nil
}

func (vm *VM) writeObccaPackage(tw *tar.Writer) error {
	startTime := time.Now()

	dockerFileContents := viper.GetString("peer.Dockerfile")
	dockerFileContents = dockerFileContents + "WORKDIR obc-ca\nRUN go install && cp obcca.yaml $GOPATH/bin\n"
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tw.Write([]byte(dockerFileContents))
	err := cutil.WriteGopathSrc(tw, "")
	if err != nil {
		return fmt.Errorf("Error writing obcca package contents: %s", err)
	}
	return nil
}

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

	var err error
	spec.ChaincodeID.Name, err = generateHashcode(spec, tw)
	if err != nil {
		tw.Close()
		gw.Close()
		return nil, fmt.Errorf("Error generating hashcode: %s", err)
	}
	err = writeChaincodePackage(spec, tw)
	if err != nil {
		tw.Close()
		gw.Close()
		return nil, fmt.Errorf("Error writing chaincode package: %s", err)
	}

	tw.Close()
	gw.Close()

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

//tw is expected to have the chaincode in it from GenerateHashcode. This method
//will just package rest of the bytes
func writeChaincodePackage(spec *pb.ChaincodeSpec, tw *tar.Writer) error {

	var urlLocation string
	if strings.HasPrefix(spec.ChaincodeID.Path, "http://") {
		urlLocation = spec.ChaincodeID.Path[7:]
	} else if strings.HasPrefix(spec.ChaincodeID.Path, "https://") {
		urlLocation = spec.ChaincodeID.Path[8:]
	} else {
		urlLocation = spec.ChaincodeID.Path
	}

	if urlLocation == "" {
		return fmt.Errorf("empty url location")
	}

	if strings.LastIndex(urlLocation, "/") == len(urlLocation)-1 {
		urlLocation = urlLocation[:len(urlLocation)-1]
	}
	toks := strings.Split(urlLocation, "/")
	if toks == nil || len(toks) == 0 {
		return fmt.Errorf("cannot get path components from %s", urlLocation)
	}

	chaincodeGoName := toks[len(toks)-1]
	if chaincodeGoName == "" {
		return fmt.Errorf("could not get chaincode name from path %s", urlLocation)
	}

	//let the executable's name be chaincode ID's name
	newRunLine := fmt.Sprintf("RUN go install %s && cp src/github.com/openblockchain/obc-peer/openchain.yaml $GOPATH/bin && mv $GOPATH/bin/%s $GOPATH/bin/%s", urlLocation, chaincodeGoName, spec.ChaincodeID.Name)

	dockerFileContents := fmt.Sprintf("%s\n%s", viper.GetString("chaincode.golang.Dockerfile"), newRunLine)
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	//Make headers identical by using zero time
	var zeroTime time.Time
	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: zeroTime, AccessTime: zeroTime, ChangeTime: zeroTime})
	tw.Write([]byte(dockerFileContents))
	err := writeGopathSrc(tw, urlLocation)
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
	err := writeGopathSrc(tw, "")
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
	err := writeGopathSrc(tw, "")
	if err != nil {
		return fmt.Errorf("Error writing obcca package contents: %s", err)
	}
	return nil
}

func writeGopathSrc(tw *tar.Writer, excludeDir string) error {
	gopath := os.Getenv("GOPATH")
	if strings.LastIndex(gopath, "/") == len(gopath)-1 {
		gopath = gopath[:len(gopath)]
	}
	rootDirectory := fmt.Sprintf("%s%s%s", os.Getenv("GOPATH"), string(os.PathSeparator), "src")
	vmLogger.Info("rootDirectory = %s", rootDirectory)

	//append "/" if necessary
	if excludeDir != "" && strings.LastIndex(excludeDir, "/") < len(excludeDir)-1 {
		excludeDir = excludeDir + "/"
	}

	rootDirLen := len(rootDirectory)
	walkFn := func(path string, info os.FileInfo, err error) error {

		// If path includes .git, ignore
		if strings.Contains(path, ".git") {
			return nil
		}

		if info.Mode().IsDir() {
			return nil
		}

		//exclude any files with excludeDir prefix. They should already be in the tar
		if excludeDir != "" && strings.Index(path, excludeDir) == rootDirLen+1 { //1 for "/"
			return nil
		}
		// Because of scoping we can reference the external rootDirectory variable
		newPath := fmt.Sprintf("src%s", path[rootDirLen:])
		//newPath := path[len(rootDirectory):]
		if len(newPath) == 0 {
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
		//Let's take the variance out of the tar, make headers identical everywhere by using zero time
		var zeroTime time.Time
		h.AccessTime = zeroTime
		h.ModTime = zeroTime
		h.ChangeTime = zeroTime
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

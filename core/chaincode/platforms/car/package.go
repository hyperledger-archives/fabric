package car

import (
	"archive/tar"
	"fmt"
	cutil "github.com/hyperledger/fabric/core/container/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Find the instance of "name" installed on the host's $PATH and inject it into the package
// This is a bit naive in that it assumes that the file returned from "which" is all that is
// required to run the binary in a different environment.  If the binary happened to have
// dependencies (such as to .so libraries or /etc/ files, etc) this probably wouldn't work
// as expected.  However, our intended use cases involves binaries generated in golang and
// clojure, both of which have a tendency to create stand-alone binaries.  Therefore, this
// is still helpful despite being a bit dumb.
func writeExecutableToPackage(name string, tw *tar.Writer) error {
	cmd := exec.Command("which", name)
	path, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Error determining %s path dynamically", name)
	}

	return cutil.WriteFileToPackage(strings.Trim(string(path), "\n"), "bin/"+name, tw)
}

func download(path string) (string, error) {
	if strings.HasPrefix(path, "http://") {
		// The file is remote, so we need to download it to a temporary location first

		var tmp *os.File
		var err error
		tmp, err = ioutil.TempFile("", "car")
		if err != nil {
			return "", fmt.Errorf("Error creating temporary file: %s", err)
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()

		resp, err := http.Get(path)
		if err != nil {
			return "", fmt.Errorf("Error with HTTP GET: %s", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(tmp, resp.Body)
		if err != nil {
			return "", fmt.Errorf("Error downloading bytes: %s", err)
		}

		return tmp.Name(), nil
	}

	return path, nil
}

//----------------------------------------------------------------
// Platform::WritePackage
//----------------------------------------------------------------
// Satisfies the platform interface for generating a docker package
// that encapsulates the environment for a CAR based chaincode
//----------------------------------------------------------------
func (self *Platform) WritePackage(spec *pb.ChaincodeSpec, tw *tar.Writer) error {

	path, err := download(spec.ChaincodeID.Path)
	if err != nil {
		return err
	}

	spec.ChaincodeID.Name, err = generateHashcode(spec, path)
	if err != nil {
		return fmt.Errorf("Error generating hashcode: %s", err)
	}

	buf := make([]string, 0)

	//let the executable's name be chaincode ID's name
	buf = append(buf, viper.GetString("chaincode.car.Dockerfile"))
	buf = append(buf, "COPY bin/* /usr/local/bin/")
	buf = append(buf, "COPY package.car /tmp/package.car")
	buf = append(buf, fmt.Sprintf("RUN chaintool buildcar /tmp/package.car -o $GOPATH/bin/%s && rm /tmp/package.car", spec.ChaincodeID.Name))

	dockerFileContents := strings.Join(buf, "\n")
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	//Make headers identical by using zero time
	var zeroTime time.Time
	tw.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: zeroTime, AccessTime: zeroTime, ChangeTime: zeroTime})
	tw.Write([]byte(dockerFileContents))

	err = cutil.WriteFileToPackage(path, "package.car", tw)
	if err != nil {
		return err
	}

	err = writeExecutableToPackage("protoc-gen-go", tw)
	if err != nil {
		return err
	}
	err = writeExecutableToPackage("chaintool", tw)
	if err != nil {
		return err
	}

	err = cutil.WriteGopathSrc(tw, "")
	if err != nil {
		return fmt.Errorf("Error writing Chaincode package contents: %s", err)
	}

	return nil
}

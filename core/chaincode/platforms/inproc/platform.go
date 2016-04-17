package inproc

import (
	"archive/tar"
	"fmt"
	pb "github.com/hyperledger/fabric/protos"
	"net/url"
	"os"
	"path/filepath"
)

type Platform struct {
}

// Returns whether the given file or directory exists or not
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func (self *Platform) ValidateSpec(spec *pb.ChaincodeSpec) error {
	url, err := url.Parse(spec.ChaincodeID.Path)
	if err != nil || url == nil {
		return fmt.Errorf("invalid path: %s", err)
	}

	pathToCheck := filepath.Join(os.Getenv("GOPATH"), "src", spec.ChaincodeID.Path)
	exists, err := pathExists(pathToCheck)
	if err != nil {
		return fmt.Errorf("Error validating chaincode path: %s", err)
	}
	if !exists {
		return fmt.Errorf("Path to chaincode does not exist: %s", spec.ChaincodeID.Path)
	}
	return nil
}

func (self *Platform) WritePackage(spec *pb.ChaincodeSpec, tw *tar.Writer) error {
	//nothing to do
	return nil
}

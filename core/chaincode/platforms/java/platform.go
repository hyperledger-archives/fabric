package java

import (
	"archive/tar"
	"fmt"
	"net/url"
	"os"

	pb "github.com/hyperledger/fabric/protos"
	//	"path/filepath"
)

// Platform for java chaincodes in java
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

//ValidateSpec validates the java chaincode specs
func (javaPlatform *Platform) ValidateSpec(spec *pb.ChaincodeSpec) error {
	url, err := url.Parse(spec.ChaincodeID.Path)
	if err != nil || url == nil {
		return fmt.Errorf("invalid path: %s", err)
	}

	//we have no real good way of checking existence of remote urls except by downloading and testing
	//which we do later anyway. But we *can* - and *should* - test for existence of local paths.
	//Treat empty scheme as a local filesystem path
	//	if url.Scheme == "" {
	//		pathToCheck := filepath.Join(os.Getenv("GOPATH"), "src", spec.ChaincodeID.Path)
	//		exists, err := pathExists(pathToCheck)
	//		if err != nil {
	//			return fmt.Errorf("Error validating chaincode path: %s", err)
	//		}
	//		if !exists {
	//			return fmt.Errorf("Path to chaincode does not exist: %s", spec.ChaincodeID.Path)
	//		}
	//	}
	return nil
}

// WritePackage writes the java chaincode package
func (javaPlatform *Platform) WritePackage(spec *pb.ChaincodeSpec, tw *tar.Writer) error {

	var err error
	spec.ChaincodeID.Name, err = generateHashcode(spec, tw)
	if err != nil {
		return err
	}

	err = writeChaincodePackage(spec, tw)
	if err != nil {
		return err
	}

	return nil
}

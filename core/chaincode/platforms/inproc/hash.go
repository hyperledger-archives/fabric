package inproc

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
)

//hashFilesInDir computes h=hash(h,file bytes) for each file in a directory
//Directory entries are traversed recursively. In the end a single
//hash value is returned for the entire directory structure
func hashFilesInDir(rootDir string, dir string, hash []byte) ([]byte, error) {
	//ReadDir returns sorted list of files in dir
	fis, err := ioutil.ReadDir(rootDir + "/" + dir)
	if err != nil {
		return hash, fmt.Errorf("ReadDir failed %s\n", err)
	}
	for _, fi := range fis {
		name := fmt.Sprintf("%s/%s", dir, fi.Name())
		if fi.IsDir() {
			var err error
			hash, err = hashFilesInDir(rootDir, name, hash)
			if err != nil {
				return hash, err
			}
			continue
		}
		fqp := rootDir + "/" + name
		buf, err := ioutil.ReadFile(fqp)
		if err != nil {
			fmt.Printf("Error reading %s\n", err)
			return hash, err
		}

		newSlice := make([]byte, len(hash)+len(buf))
		copy(newSlice[len(buf):], hash[:])
		//hash = md5.Sum(newSlice)
		hash = util.ComputeCryptoHash(newSlice)
	}
	return hash, nil
}

func isCodeExist(tmppath string) error {
	file, err := os.Open(tmppath)
	if err != nil {
		return fmt.Errorf("Download failer %s", err)
	}

	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("could not stat file %s", err)
	}

	if !fi.IsDir() {
		return fmt.Errorf("file %s is not dir\n", file.Name())
	}

	return nil
}

func getCodeFromFS(path string) (codegopath string, err error) {
	env := os.Environ()
	var gopath string
	for _, v := range env {
		if strings.Index(v, "GOPATH=") == 0 {
			p := strings.SplitAfter(v, "GOPATH=")
			gopath = p[1]
			break
		}
	}

	if gopath == "" {
		return
	}

	codegopath = gopath

	return
}

//generateHashcode gets hashcode of the code under path. If path is a HTTP(s) url
//it downloads the code first to compute the hash.
//NOTE: for dev mode, user builds and runs chaincode manually. The name provided
//by the user is equivalent to the path. This method will treat the name
//as codebytes and compute the hash from it. ie, user cannot run the chaincode
//with the same (name, ctor, args)
func generateHashcode(spec *pb.ChaincodeSpec) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("Cannot generate hashcode from nil spec")
	}

	chaincodeID := spec.ChaincodeID
	if chaincodeID == nil || chaincodeID.Path == "" {
		return "", fmt.Errorf("Cannot generate hashcode from empty chaincode path")
	}

	ctor := spec.CtorMsg
	if ctor == nil {
		return "", fmt.Errorf("Cannot generate hashcode from nil ctor")
	}

	//code root will point to the directory where the code exists
	//in the case of http it will be a temporary dir that
	//will have to be deleted
	var codegopath string

	var ishttp bool
	defer func() {
		if ishttp && codegopath != "" {
			os.RemoveAll(codegopath)
		}
	}()

	path := chaincodeID.Path

	actualcodepath := path
	codegopath, err := getCodeFromFS(path)

	if err != nil {
		return "", fmt.Errorf("Error getting code %s", err)
	}

	tmppath := codegopath + "/src/" + actualcodepath
	if err = isCodeExist(tmppath); err != nil {
		return "", fmt.Errorf("code does not exist %s", err)
	}

	hash := util.GenerateHashFromSignature(actualcodepath, ctor.Function, ctor.Args)

	hash, err = hashFilesInDir(codegopath+"/src/", actualcodepath, hash)
	if err != nil {
		return "", fmt.Errorf("Could not get hashcode for %s - %s\n", path, err)
	}

	return hex.EncodeToString(hash[:]), nil
}

package container

import (
	"archive/tar"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

func addFile(tw *tar.Writer, path string, info os.FileInfo, fbytes []byte) error {
	h, err := tar.FileInfoHeader(info, path)
	if err != nil {
		return fmt.Errorf("Error getting FileInfoHeader: %s", err)
	}
	//Let's take the variance out of the tar, make headers identical by using zero time
	var zeroTime time.Time
	h.AccessTime = zeroTime
	h.ModTime = zeroTime
	h.ChangeTime = zeroTime
	h.Name = path
	if err = tw.WriteHeader(h); err != nil {
		return fmt.Errorf("Error writing header: %s", err)
	}
	rdr := bytes.NewReader(fbytes)
	if _, err := io.Copy(tw, rdr); err != nil {
		return fmt.Errorf("Error copying file : %s", err)
	}
	return nil
}

//hashFilesInDir computes h=hash(h,file bytes) for each file in a directory
//Directory entries are traversed recursively. In the end a single
//hash value is returned for the entire directory structure
func hashFilesInDir(rootDir string, dir string, hash []byte, tw *tar.Writer) ([]byte, error) {
	//ReadDir returns sorted list of files in dir
	fis, err := ioutil.ReadDir(rootDir + "/" + dir)
	if err != nil {
		return hash, fmt.Errorf("ReadDir failed %s\n", err)
	}
	for _, fi := range fis {
		name := fmt.Sprintf("%s/%s", dir, fi.Name())
		if fi.IsDir() {
			var err error
			hash, err = hashFilesInDir(rootDir, name, hash, tw)
			if err != nil {
				return hash, err
			}
			continue
		}
		buf, err := ioutil.ReadFile(rootDir + "/" + name)
		if err != nil {
			fmt.Printf("Error reading %s\n", err)
			return hash, err
		}

		newSlice := make([]byte, len(hash)+len(buf))
		copy(newSlice[len(buf):], hash[:])
		//hash = md5.Sum(newSlice)
		hash = util.ComputeCryptoHash(newSlice)

		if tw != nil {
			if err = addFile(tw, "src/"+name, fi, buf); err != nil {
				return hash, fmt.Errorf("Error adding file to tar %s", err)
			}
		}
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

func getCodeFromHTTP(path string) (codegopath string, err error) {
	codegopath = ""
	err = nil

	env := os.Environ()
	var newgopath string
	var origgopath string
	var gopathenvIndex int
	for i, v := range env {
		if strings.Index(v, "GOPATH=") == 0 {
			p := strings.SplitAfter(v, "GOPATH=")
			origgopath = p[1]
			newgopath = origgopath + "/_usercode_"
			gopathenvIndex = i
			break
		}
	}

	if newgopath == "" {
		err = fmt.Errorf("GOPATH not defined")
		return
	}

	//ignore errors.. _usercode_ might exist. TempDir will catch any other errors
	os.Mkdir(newgopath, 0755)    

	if codegopath, err = ioutil.TempDir(newgopath, ""); err != nil {
		err = fmt.Errorf("could not create tmp dir under %s(%s)", newgopath, err)
		return
	}
	
	// If we don't copy the obc-peer code into the temp dir with the chaincode,
	// go get will have to redownload obc-peer, which requires credentials.
	cmd := exec.Command("cp", "-r", origgopath + "/src", codegopath + "/src")
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return
	}

	env[gopathenvIndex] = "GOPATH=" + codegopath

	cmd = exec.Command("go", "get", path)
	cmd.Env = env
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return
	}

	return
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

//name could be ChaincodeID.Name or ChaincodeID.Path
func generateHashFromSignature(path string, ctor string, args []string) []byte {
	fargs := ctor
	if args != nil {
		for _, str := range args {
			fargs = fargs + str
		}
	}
	cbytes := []byte(path + fargs)

	b := make([]byte, len(cbytes))
	copy(b, cbytes)
	hash := util.ComputeCryptoHash(b)
	return hash
}

//generateHashcode gets hashcode of the code under path. If path is a HTTP(s) url
//it downloads the code first to compute the hash.
//NOTE: for dev mode, user builds and runs chaincode manually. The name provided
//by the user is equivalent to the path. This method will treat the name
//as codebytes and compute the hash from it. ie, user cannot run the chaincode
//with the same (name, ctor, args)
func generateHashcode(spec *pb.ChaincodeSpec, tw *tar.Writer) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("Cannot generate hashcode from nil spec")
	}

	chaincodeID := spec.ChaincodeID
	if chaincodeID == nil || chaincodeID.Path == "" {
		return "", fmt.Errorf("Cannot generate hashcode from empty chaincode path")
	}

	ctor := spec.CtorMsg
	if ctor == nil || ctor.Function == "" {
		return "", fmt.Errorf("Cannot generate hashcode from empty ctor")
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

	var err error
	var actualcodepath string
	if strings.HasPrefix(path, "http://") {
		ishttp = true
		actualcodepath = path[7:]
		codegopath, err = getCodeFromHTTP(actualcodepath)
	} else if strings.HasPrefix(path, "https://") {
		ishttp = true
		actualcodepath = path[8:]
		codegopath, err = getCodeFromHTTP(actualcodepath)
	} else {
		actualcodepath = path
		codegopath, err = getCodeFromFS(path)
	}

	if err != nil {
		return "", fmt.Errorf("Error getting code %s", err)
	}

	tmppath := codegopath + "/src/" + actualcodepath
	if err = isCodeExist(tmppath); err != nil {
		return "", fmt.Errorf("code does not exist %s", err)
	}

	hash := generateHashFromSignature(actualcodepath, ctor.Function, ctor.Args)

	hash, err = hashFilesInDir(codegopath+"/src/", actualcodepath, hash, tw)
	if err != nil {
		return "", fmt.Errorf("Could not get hashcode for %s - %s\n", path, err)
	}

	return hex.EncodeToString(hash[:]), nil
}

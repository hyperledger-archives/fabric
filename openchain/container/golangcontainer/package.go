package golangcontainer

import (
	"archive/tar"
	"fmt"
	"github.com/spf13/viper"
	"strings"
	"time"

	cutil "github.com/openblockchain/obc-peer/openchain/container/util"
	pb "github.com/openblockchain/obc-peer/protos"
)

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
	err := cutil.WriteGopathSrc(tw, urlLocation)
	if err != nil {
		return fmt.Errorf("Error writing Chaincode package contents: %s", err)
	}
	return nil
}

func WritePackage(spec *pb.ChaincodeSpec, tw *tar.Writer) error {

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

package car

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
)

//generateHashcode gets hashcode of the code under path. If path is a HTTP(s) url
//it downloads the code first to compute the hash.
//NOTE: for dev mode, user builds and runs chaincode manually. The name provided
//by the user is equivalent to the path. This method will treat the name
//as codebytes and compute the hash from it. ie, user cannot run the chaincode
//with the same (name, ctor, args)
func generateHashcode(spec *pb.ChaincodeSpec, path string) (string, error) {

	ctor := spec.CtorMsg
	if ctor == nil || ctor.Function == "" {
		return "", fmt.Errorf("Cannot generate hashcode from empty ctor")
	}

	hash := util.GenerateHashFromSignature(spec.ChaincodeID.Path, ctor.Function, ctor.Args)

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Error reading file: %s", err)
	}

	newSlice := make([]byte, len(hash)+len(buf))
	copy(newSlice[len(buf):], hash[:])
	//hash = md5.Sum(newSlice)
	hash = util.ComputeCryptoHash(newSlice)

	return hex.EncodeToString(hash[:]), nil
}

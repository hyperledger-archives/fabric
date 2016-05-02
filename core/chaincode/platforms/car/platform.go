package car

import (
	pb "github.com/hyperledger/fabric/protos"
)

type Platform struct {
}

//----------------------------------------------------------------
// Platform::ValidateSpec
//----------------------------------------------------------------
// Validates the chaincode specification for CAR types to satisfy
// the platform interface.  This chaincode type currently doesn't
// require anything specific so we just implicitly approve any spec
//----------------------------------------------------------------
func (self *Platform) ValidateSpec(spec *pb.ChaincodeSpec) error {
	return nil
}

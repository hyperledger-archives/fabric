package validator

import (
	pb "github.com/openblockchain/obc-peer/protos"
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/peer"
)


// Errors

var ErrRegistrationRequired error = errors.New("Validator Not Registered to the Membership Service.")
var ErrModuleNotInitialized = errors.New("Validator Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Validator Security Module Already Initilized.")


// Public Struct


type Validator struct {
	*peer.Peer

	isInitialized bool
}


// Public Methods


// Register is used to register this validator to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (validator *Validator) Register(userId, pwd string) (error) {
	return nil
}

// Init initializes this validator by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (validator *Validator) Init() (error) {
	if (validator.isInitialized) {
		return ErrModuleAlreadyInitialized
	}

	validator.isInitialized = true;

	return nil
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification)
// In addition, the method verifies that the validator
// is allowed to validate the transaction and if this is the case
// the method decrypts all the relevant fields.
func (validator *Validator) TransactionPreValidation(tx *pb.Transaction) (*pb.Transaction, error) {
	if (!validator.isInitialized) {
		return nil, ErrModuleNotInitialized
	}

	return tx, nil
}

package validator

import (
	pb "github.com/openblockchain/obc-peer/protos"
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/peer"
"crypto/rand"
)


// Errors

var ErrRegistrationRequired error = errors.New("Validator Not Registered to the Membership Service.")
var ErrModuleNotInitialized = errors.New("Validator Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Validator Security Module Already Initilized.")


// Public Struct


type Validator struct {
	*peer.Peer

	isInitialized bool
	id []byte
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

	// Init field

	// id is initialized to a random value. Later on,
	// id will be initialized as the hash of the enrollment certificate
	validator.id = make([]byte, 48)
	_, err := rand.Read(validator.id)
	if err != nil {
		return err
	}

	// Initialisation complete
	validator.isInitialized = true;

	return nil
}

// GetID returns this validator's identifier
func (validator *Validator) GetID() ([]byte) {
	// Clone id to avoid exposure of internal data structure
	clone := make([]byte, len(validator.id))
	copy(clone, validator.id)

	return clone
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification).
func (validator *Validator) TransactionPreValidation(tx *pb.Transaction) (*pb.Transaction, error) {
	if (!validator.isInitialized) {
		return nil, ErrModuleNotInitialized
	}

	return tx, nil
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification). If this is the case,
// the method prepares the transaction to be executed.
func (validator *Validator) TransactionPreExecution(tx *pb.Transaction) (*pb.Transaction, error) {
	if (!validator.isInitialized) {
		return nil, ErrModuleNotInitialized
	}

	return tx, nil
}

func (validator *Validator) Sign([]byte) ([]byte, error) {

}

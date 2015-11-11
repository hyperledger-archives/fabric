package peer


import (
	pb "github.com/openblockchain/obc-peer/protos"
	"errors"
)

// Errors

var ErrRegistrationRequired error = errors.New("Peer Not Registered to the Membership Service.")
var ErrModuleNotInitialized = errors.New("Peer Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Peer Security Module Already Initilized.")


// Public Struct

type Peer struct {
	isInitialized bool
}


// Public Methods

// Register is used to register this peer to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (peer *Peer) Register(userId, pwd string) (error) {
	return nil
}

// Init initializes this peer by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (peer *Peer) Init() (error) {
	if (peer.isInitialized) {
		return ErrModuleAlreadyInitialized
	}

	peer.isInitialized = true;

	return nil
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification)
func (peer *Peer) TransactionPreValidation(tx *pb.Transaction) (*pb.Transaction, error) {
	if (!peer.isInitialized) {
		return nil, ErrModuleNotInitialized
	}

	return tx, nil
}

package client

import (
	pb "github.com/openblockchain/obc-peer/protos"
	"errors"
)

// Errors

var ErrRegistrationRequired error = errors.New("Client Not Registered to the Membership Service.")
var ErrModuleNotInitialized error = errors.New("Client Security Module Not Initilized.")
var ErrModuleAlreadyInitialized error = errors.New("Client Security Module Already Initilized.")



// Public Struct

type Client struct {
	isInitialized bool
}


// Public Methods


// Register is used to register this client to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (client *Client) Register(userId, pwd string) (error) {
	return nil
}

// Init initializes this client by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (client *Client) Init() (error) {
	if (client.isInitialized) {
		return ErrModuleAlreadyInitialized
	}

	client.isInitialized = true;
	return nil
}

// NewChainletDeployTransaction is used to deploy chaincode.
func (client *Client) NewChainletDeployTransaction(chainletDeploymentSpec *pb.ChainletDeploymentSpec, uuid string) (*pb.Transaction, error) {
	if (!client.isInitialized) {
		return nil, ErrModuleNotInitialized
	}

	return pb.NewChainletDeployTransaction(chainletDeploymentSpec, uuid)
}

// NewChainletInvokeTransaction is used to invoke chaincode's functions.
func (client *Client) NewChainletInvokeTransaction(chaincodeInvocation *pb.ChaincodeInvocation, uuid string) (*pb.Transaction, error) {
	if (!client.isInitialized) {
		return nil, ErrModuleNotInitialized
	}

	return pb.NewChainletInvokeTransaction(chaincodeInvocation, uuid)
}

// Private Methods

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *Client) checkTransaction(*pb.Transaction) (error) {
	if (!client.isInitialized) {
		return ErrModuleNotInitialized
	}

	return nil
}
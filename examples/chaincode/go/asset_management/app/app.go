package main

import (
	"github.com/hyperledger/fabric/core/crypto"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/op/go-logging"
	"google.golang.org/grpc"
	"os"
	"reflect"
	"time"
	"fmt"
)

var (
	// Logging
	appLogger = logging.MustGetLogger("app")

	// NVP related objects
	peerClientConn *grpc.ClientConn
	serverClient   pb.PeerClient

	// Alice is the deployer
	alice crypto.Client

	// Bob is the administrator
	bob     crypto.Client
	bobCert crypto.CertificateHandler

	// Charlie and Dave are owners
	charlie     crypto.Client
	charlieCert crypto.CertificateHandler

	dave     crypto.Client
	daveCert crypto.CertificateHandler
)

func deploy() (err error) {
	appLogger.Debug("------------- Alice wants to assign the administrator role to Bob;")
	// Deploy:
	// 1. Alice is the deployer of the chaincode;
	// 2. Alice wants to assign the administrator role to Bob;
	// 3. Alice obtains, via an out-of-band channel, a TCert of Bob, let us call this certificate *BobCert*;

	bobCert, err = bob.GetTCertificateHandlerNext()
	if err != nil {
		appLogger.Error("Failed getting Bob TCert [%s]", err)
		return
	}

	// 4. Alice constructs a deploy transaction, as described in *application-ACL.md*,  setting the transaction
	// metadata to *DER(CharlieCert)*.
	// 5. Alice submits th	e transaction to the fabric network.
	resp, err := deployInternal(alice, bobCert)
	if err != nil {
		appLogger.Error("Failed deploying [%s]", err)
		return
	}
	appLogger.Debug("Resp [%s]", resp.String())
	appLogger.Debug("Chaincode NAME: [%s]-[%s]", chaincodeName, string(resp.Msg))

	appLogger.Debug("Wait 30 seconds")
	time.Sleep(30 * time.Second)

	appLogger.Debug("------------- Done!")
	return
}

func assignOwnership() (err error) {
	appLogger.Debug("------------- Bob wants to assign the asset 'Picasso' to Charlie...")

	// 1. Bob is the administrator of the chaincode;
	// 2. Bob wants to assign the asset 'Picasso' to Charlie;
	// 3. Bob obtains, via an out-of-band channel, a TCert of Charlie, let us call this certificate *CharlieCert*;

	// Administrator assigns ownership of Picasso to Alice
	charlieCert, err = charlie.GetTCertificateHandlerNext()
	if err != nil {
		appLogger.Error("Failed getting Charlie TCert [%s]", err)
		return
	}

	// 4. Bob constructs an execute transaction, as described in *application-ACL.md*, to invoke the *assign*
	// function passing as parameters *('Picasso', DER(CharlieCert))*.
	// 5. Bob submits the transaction to the fabric network.

	resp, err := assignOwnershipInternal(bob, bobCert, "Picasso", charlieCert)
	if err != nil {
		appLogger.Error("Failed assigning ownership [%s]", err)
		return
	}
	appLogger.Debug("Resp [%s]", resp.String())

	appLogger.Debug("Wait 30 seconds")
	time.Sleep(30 * time.Second)

	// Check the owner of 'Picasso". It should be charlie
	appLogger.Debug("Query....")
	queryTx, theOwnerIs, err := whoIsTheOwner(bob, "Picasso")
	if err != nil {
		return
	}
	appLogger.Debug("Resp [%s]", theOwnerIs.String())
	appLogger.Debug("Query....done")

	var res []byte
	if confidentialityOn {
		// Decrypt result
		res, err = bob.DecryptQueryResult(queryTx, theOwnerIs.Msg)
		if err != nil {
			appLogger.Error("Failed decrypting result [%s]", err)
			return
		}
	} else {
		res = theOwnerIs.Msg
	}

	if !reflect.DeepEqual(res, charlieCert.GetCertificate()) {
		appLogger.Error("Charlie is not the owner.")

		appLogger.Debug("Query result  : [% x]", res)
		appLogger.Debug("Charlie's cert: [% x]", charlieCert.GetCertificate())

		return fmt.Errorf("Charlie is not the owner.")
	}
	appLogger.Debug("Charlie is the owner!")

	appLogger.Debug("Wait 30 seconds...")
	time.Sleep(30 * time.Second)

	appLogger.Debug("------------- Done!")
	return
}

func transferOwnership() (err error) {
	appLogger.Debug("------------- Charlie wants to transfer the ownership of 'Picasso' to Dave...")

	// 1. Charlie is the owner of 'Picasso';
	// 2. Charlie wants to transfer the ownership of 'Picasso' to Dave;
	// 3. Charlie obtains, via an out-of-band channel, a TCert of Dave, let us call this certificate *DaveCert*;
	daveCert, err = dave.GetTCertificateHandlerNext()
	if err != nil {
		appLogger.Error("Failed getting Dave TCert [%s]", err)
		return
	}

	// 4. Charlie constructs an execute transaction, as described in *application-ACL.md*, to invoke the *transfer*
	// function passing as parameters *('Picasso', DER(DaveCert))*.
	// 5. Charlie submits the transaction to the fabric network.

	resp, err := transferOwnershipInternal(charlie, charlieCert, "Picasso", daveCert)
	if err != nil {
		return
	}
	appLogger.Debug("Resp [%s]", resp.String())

	appLogger.Debug("Wait 30 seconds")
	time.Sleep(30 * time.Second)

	appLogger.Debug("Query....")
	queryTx, theOwnerIs, err := whoIsTheOwner(charlie, "Picasso")
	if err != nil {
		return
	}
	appLogger.Debug("Resp [%s]", theOwnerIs.String())
	appLogger.Debug("Query....done")

	var res []byte
	if confidentialityOn {
		// Decrypt result
		res, err = charlie.DecryptQueryResult(queryTx, theOwnerIs.Msg)
		if err != nil {
			appLogger.Error("Failed decrypting result [%s]", err)
			return
		}
	} else {
		res = theOwnerIs.Msg
	}

	if !reflect.DeepEqual(res, daveCert.GetCertificate()) {
		appLogger.Error("Dave is not the owner.")

		appLogger.Debug("Query result  : [% x]", res)
		appLogger.Debug("Dave's cert: [% x]", daveCert.GetCertificate())

		return fmt.Errorf("Dave is not the owner.")
	}

	appLogger.Debug("------------- Done!")
	return
}

func testAssetManagementChaincode() (err error) {
	// Deploy
	err = deploy()
	if err != nil {
		appLogger.Error("Failed deploying [%s]", err)
		return
	}

	// Assign
	err = assignOwnership()
	if err != nil {
		appLogger.Error("Failed assigning ownership [%s]", err)
		return
	}

	// Transfer
	err = transferOwnership()
	if err != nil {
		appLogger.Error("Failed transfering ownership [%s]", err)
		return
	}

	appLogger.Debug("Dave is the owner!")

	return
}

func main() {
	// Initialize a non-validating peer whose role is to submit
	// transactions to the fabric network.
	// A 'core.yaml' file is assumed to be available in the working directory.
	if err := initNVP(); err != nil {
		appLogger.Debug("Failed initiliazing NVP [%s]", err)
		os.Exit(-1)
	}

	// Enable fabric 'confidentiality'
	confidentiality(true)

	// Exercise the 'asset_management' chaincode
	if err := testAssetManagementChaincode(); err != nil {
		appLogger.Debug("Failed testing asset management chaincode [%s]", err)
		os.Exit(-2)
	}
}

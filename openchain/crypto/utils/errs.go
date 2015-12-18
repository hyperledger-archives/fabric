package utils

import "errors"

var (
	// ErrRegistrationRequired Registration to the Membership Service required.
	ErrRegistrationRequired        = errors.New("Registration to the Membership Service required.")

	// ErrNotInitialized Initialization required
	ErrNotInitialized              = errors.New("Initialization required.")

	// ErrAlreadyInitialized Already initialized
	ErrAlreadyInitialized          = errors.New("Already initialized.")

	// ErrAlreadyRegistered Already registered
	ErrAlreadyRegistered           = errors.New("Already registered.")

	// ErrTransactionMissingCert Transaction missing certificate or signature
	ErrTransactionMissingCert      = errors.New("Transaction missing certificate or signature.")

	// ErrInvalidTransactionSignature Invalid Transaction Signature
	ErrInvalidTransactionSignature = errors.New("Invalid Transaction Signature.")

	// ErrTransactionCertificate Missing Transaction Certificate
	ErrTransactionCertificate      = errors.New("Missing Transaction Certificate.")

	// ErrTransactionSignature Missing Transaction Signature
	ErrTransactionSignature        = errors.New("Missing Transaction Signature.")

	// ErrInvalidSignature Invalid Signature
	ErrInvalidSignature            = errors.New("Invalid Signature.")

	// ErrInvalidReference Invalid reference
	ErrInvalidReference            = errors.New("Invalid reference.")

	// ErrNotImplemented Not implemented
	ErrNotImplemented              = errors.New("Not implemented.")

	// ErrKeyStoreAlreadyInitialized Keystore already Initilized
	ErrKeyStoreAlreadyInitialized  = errors.New("Keystore already Initilized.")

	// ErrEncrypt Encryption failed
	ErrEncrypt                     = errors.New("Encryption failed.")

	// ErrDecrypt Decryption failed
	ErrDecrypt                     = errors.New("Decryption failed.")

	// ErrDirrentChaincodeID ChaincodeIDs are different
	ErrDirrentChaincodeID          = errors.New("ChaincodeIDs are different.")

	// ErrInvalidConfidentialityLevel Invalid confidentiality level
	ErrInvalidConfidentialityLevel = errors.New("Invalid confidentiality level")
)

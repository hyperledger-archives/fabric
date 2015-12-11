package utils

import "errors"

var (
	ErrRegistrationRequired        error = errors.New("Registration to the Membership Service required.")
	ErrNotInitialized              error = errors.New("Initilized required.")
	ErrAlreadyInitialized          error = errors.New("Already initilized.")
	ErrAlreadyRegistered           error = errors.New("Already registered.")
	ErrTransactionMissingCert      error = errors.New("Transaction missing certificate or signature.")
	ErrInvalidTransactionSignature error = errors.New("Invalid Transaction Signature.")
	ErrTransactionCertificate      error = errors.New("Missing Transaction Certificate.")
	ErrTransactionSignature        error = errors.New("Missing Transaction Signature.")
	ErrInvalidSignature            error = errors.New("Invalid Signature.")
	ErrInvalidReference            error = errors.New("Invalid reference.")

	ErrKeyStoreAlreadyInitialized error = errors.New("Keystore already Initilized.")
)

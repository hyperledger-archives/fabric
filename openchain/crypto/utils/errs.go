/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package utils

import "errors"

var (
	// ErrRegistrationRequired Registration to the Membership Service required.
	ErrRegistrationRequired = errors.New("Registration to the Membership Service required.")

	// ErrNotInitialized Initialization required
	ErrNotInitialized = errors.New("Initialization required.")

	// ErrAlreadyInitialized Already initialized
	ErrAlreadyInitialized = errors.New("Already initialized.")

	// ErrAlreadyRegistered Already registered
	ErrAlreadyRegistered = errors.New("Already registered.")

	// ErrTransactionMissingCert Transaction missing certificate or signature
	ErrTransactionMissingCert = errors.New("Transaction missing certificate or signature.")

	// ErrInvalidTransactionSignature Invalid Transaction Signature
	ErrInvalidTransactionSignature = errors.New("Invalid Transaction Signature.")

	// ErrTransactionCertificate Missing Transaction Certificate
	ErrTransactionCertificate = errors.New("Missing Transaction Certificate.")

	// ErrTransactionSignature Missing Transaction Signature
	ErrTransactionSignature = errors.New("Missing Transaction Signature.")

	// ErrInvalidSignature Invalid Signature
	ErrInvalidSignature = errors.New("Invalid Signature.")

	// ErrInvalidReference Invalid reference
	ErrInvalidReference = errors.New("Invalid reference.")

	// ErrInvalidReference Invalid reference
	ErrNilArgument = errors.New("Nil argument.")

	// ErrNotImplemented Not implemented
	ErrNotImplemented = errors.New("Not implemented.")

	// ErrKeyStoreAlreadyInitialized Keystore already Initilized
	ErrKeyStoreAlreadyInitialized = errors.New("Keystore already Initilized.")

	// ErrEncrypt Encryption failed
	ErrEncrypt = errors.New("Encryption failed.")

	// ErrDecrypt Decryption failed
	ErrDecrypt = errors.New("Decryption failed.")

	// ErrDifferentChaincodeID ChaincodeIDs are different
	ErrDifferentChaincodeID = errors.New("ChaincodeIDs are different.")

	// ErrInvalidConfidentialityLevel Invalid confidentiality level
	ErrInvalidConfidentialityLevel = errors.New("Invalid confidentiality level")
)

func ErrToString(err error) string {
	if err != nil {
		return err.Error()
	}

	return "<clean>"
}

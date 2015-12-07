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

package validator

import (
	"crypto/x509"
)

func (validator *Validator) initCryptoEngine() error {
	log.Info("Initialing Crypto Engine...")

	validator.rootsCertPool = x509.NewCertPool()
	validator.enrollCerts = make(map[string]*x509.Certificate)

	// Load ECA certs chain
	if err := validator.loadECACertsChain(); err != nil {
		return err
	}

	// Load TCA certs chain
	if err := validator.loadTCACertsChain(); err != nil {
		return err
	}

	// Load enrollment secret key
	// TODO: finalize encrypted pem support
	if err := validator.loadEnrollmentKey(nil); err != nil {
		return err
	}

	// Load enrollment certificate and set validator ID
	if err := validator.loadEnrollmentCertificate(); err != nil {
		return err
	}

	// Load enrollment id
	if err := validator.loadEnrollmentID(); err != nil {
		return err
	}

	// Load enrollment chain key
	if err := validator.loadEnrollmentChainKey(nil); err != nil {
		return err
	}

	log.Info("Initialing Crypto Engine...done!")

	return nil
}

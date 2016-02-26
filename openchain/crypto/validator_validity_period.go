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

package crypto

import (
	"errors"
	"github.com/spf13/viper"
	"strconv"
	"time"

	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	obc "github.com/openblockchain/obc-peer/protos"
)

//We are temporarily disabling the validity period functionality
var allowValidityPeriodVerification = false

func validityPeriodVerificationEnabled() bool {
	// If the verification of the validity period is enabled in the configuration file return the configured value
	if viper.IsSet("peer.validator.validity-period.verification") {
		return viper.GetBool("peer.validator.validity-period.verification")
	}

	// Validity period verification is enabled by default if no configuration was specified.
	return true
}

func (validator *validatorImpl) verifyValidityPeriod(tx *obc.Transaction) (*obc.Transaction, error) {
	if tx.Cert != nil && tx.Signature != nil {

		// Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			validator.error("verifyValidityPeriod: failed unmarshalling cert %s:", err)
			return tx, err
		}

		cid := viper.GetString("pki.validity-period.chaincodeHash")

		ledger, err := ledger.GetLedger()
		if err != nil {
			validator.error("verifyValidityPeriod: failed getting access to the ledger %s:", err)
			return tx, err
		}

		vpBytes, err := ledger.GetState(cid, "system.validity.period", true)
		if err != nil {
			validator.error("verifyValidityPeriod: failed reading validity period from the ledger %s:", err)
			return tx, err
		}

		i, err := strconv.ParseInt(string(vpBytes[:]), 10, 64)
		if err != nil {
			validator.error("verifyValidityPeriod: failed to parse validity period %s:", err)
			return tx, err
		}

		vp := time.Unix(i, 0)

		var errMsg string

		// Verify the validity period of the TCert
		switch {
		case cert.NotAfter.Before(cert.NotBefore):
			errMsg = "verifyValidityPeriod: certificate validity period is invalid"
		case vp.Before(cert.NotBefore):
			errMsg = "verifyValidityPeriod: certificate validity period is in the future"
		case vp.After(cert.NotAfter):
			errMsg = "verifyValidityPeriod: certificate validity period is in the past"
		}

		if errMsg != "" {
			validator.error(errMsg)
			return tx, errors.New(errMsg)
		}
	}

	return tx, nil
}

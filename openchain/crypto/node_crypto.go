package crypto

import (
	"crypto/x509"
	ecies "github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"
)

func (node *nodeImpl) registerCryptoEngine(enrollID, enrollPWD string) error {
	node.debug("Registering node crypto engine...")

	// Init CLI
	node.eciesSPI = ecies.NewSPI()

	if err := node.initTLS(); err != nil {
		node.error("Failed initliazing TLS [%s].", err.Error())

		return err
	}

	if err := node.retrieveECACertsChain(enrollID); err != nil {
		node.error("Failed retrieveing ECA certs chain [%s].", err.Error())

		return err
	}

	if err := node.retrieveTCACertsChain(enrollID); err != nil {
		node.error("Failed retrieveing ECA certs chain [%s].", err.Error())

		return err
	}

	if err := node.retrieveEnrollmentData(enrollID, enrollPWD); err != nil {
		node.error("Failed retrieveing enrollment data [%s].", err.Error())

		return err
	}

	if err := node.retrieveTLSCertificate(enrollID, enrollPWD); err != nil {
		node.error("Failed retrieveing enrollment data: %s", err)

		return err
	}

	node.debug("Registering node crypto engine...done!")

	return nil
}

func (node *nodeImpl) initCryptoEngine() error {
	node.debug("Initializing node crypto engine...")

	// Init CLI
	node.eciesSPI = ecies.NewSPI()

	// Init certPools
	node.rootsCertPool = x509.NewCertPool()
	node.tlsCertPool = x509.NewCertPool()
	node.ecaCertPool = x509.NewCertPool()
	node.tcaCertPool = x509.NewCertPool()

	// Load ECA certs chain
	if err := node.loadECACertsChain(); err != nil {
		return err
	}

	// Load TCA certs chain
	if err := node.loadTCACertsChain(); err != nil {
		return err
	}

	// Load enrollment secret key
	if err := node.loadEnrollmentKey(); err != nil {
		return err
	}

	// Load enrollment certificate and set validator ID
	if err := node.loadEnrollmentCertificate(); err != nil {
		return err
	}

	// Load enrollment id
	if err := node.loadEnrollmentID(); err != nil {
		return err
	}

	// Load enrollment chain key
	if err := node.loadEnrollmentChainKey(); err != nil {
		return err
	}

	// Load TLS certs chain certificate
	if err := node.loadTLSCACertsChain(); err != nil {
		return err
	}

	// Load tls certificate
	if err := node.loadTLSCertificate(); err != nil {
		return err
	}

	node.debug("Initializing node crypto engine...done!")

	return nil
}

package crypto

import (
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/conf"
	"github.com/spf13/viper"
)

var (
	log = logging.MustGetLogger("crypto")
)

// Init initializes the crypto layer. It load from viper the security level
// and the logging setting.
func Init() (err error) {
	// Init log
	log.ExtraCalldepth++

	level, err := logging.LogLevel(viper.GetString("logging.crypto"))
	if err == nil {
		// No error, use the setting
		logging.SetLevel(level, "crypto")
		log.Info("Log level recognized '%s', set to %s", viper.GetString("logging.crypto"),
			logging.GetLevel("crypto"))
	} else {
		log.Warning("Log level not recognized '%s', defaulting to %s: %s", viper.GetString("logging.crypto"),
			logging.GetLevel("crypto"), err)
	}

	// Init security level

	securityLevel := 256
	if viper.IsSet("security.level") {
		ovveride := viper.GetInt("security.level")
		if ovveride != 0 {
			securityLevel = ovveride
		}
	}
	log.Debug("Working at security level [%d]", securityLevel)
	if err = conf.InitSecurityLevel(securityLevel); err != nil {
		log.Debug("Failed setting security level: [%s]", err)

		return
	}

	return
}

package utils

import (
	"fmt"
	"github.com/spf13/viper"
)

// NodeConfiguration used for testing
type NodeConfiguration struct {
	Type string
	Name string
}

// GetEnrollmentID returns the enrollment ID
func (conf *NodeConfiguration) GetEnrollmentID() string {
	value := viper.GetString(conf.Type + ".crypto.users." + conf.Name + ".enrollid")
	if value == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'peer.crypto.enrollid' is set"))
	}
	return value
}

// GetEnrollmentPWD returns the enrollment PWD
func (conf *NodeConfiguration) GetEnrollmentPWD() string {
	value := viper.GetString(conf.Type + ".crypto.users." + conf.Name + ".enrollpw")
	if value == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property 'peer.crypto.enrollid' is set"))
	}
	return value
}

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
	key := "tests.crypto.users." + conf.Name + ".enrollid"
	value := viper.GetString(key)
	if value == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property '%s' is set", key))
	}
	return value
}

// GetEnrollmentPWD returns the enrollment PWD
func (conf *NodeConfiguration) GetEnrollmentPWD() string {
	key := "tests.crypto.users." + conf.Name + ".enrollpw"
	value := viper.GetString(key)
	if value == "" {
		panic(fmt.Errorf("Enrollment id not specified in configuration file. Please check that property '%s' is set", key))
	}
	return value
}

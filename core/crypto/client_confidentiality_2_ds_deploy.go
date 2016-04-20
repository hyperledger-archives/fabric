package crypto

import (
	"fmt"
)

type headerMessageV2 struct {
	ChaincodeIDFlag string
	ChaincodeID     []byte
}

func (m *headerMessageV2) Validate() error {
	if m.ChaincodeIDFlag != "chaincodeID" {
		return fmt.Errorf("Invalid ChaincodeIDFlag Flag")
	}

	return nil
}

// To Validators

type deployValidatorsMessageV2 struct {
	Flag      string
	Chain     []byte
	Chaincode []byte
}

func (m *deployValidatorsMessageV2) Validate() error {
	if m.Flag != "depValMes" {
		return fmt.Errorf("Invalid Deploy Validators Message Flag")
	}

	return nil
}

type deployValidatorsMessageChainV2 struct {
	SkCFlag string
	SkC     []byte
}

func (m *deployValidatorsMessageChainV2) Validate() error {
	if m.SkCFlag != "skC" {
		return fmt.Errorf("Invalid skC Flag")
	}

	return nil
}

type deployValidatorsMessageChaincodeV2 struct {
	KHeaderFlag string
	KHeader     []byte
	KCodeFlag   string
	KCode       []byte
	KStateFlag  string
	KState      []byte
}

func (m *deployValidatorsMessageChaincodeV2) Validate() error {
	if m.KCodeFlag != "code" {
		return fmt.Errorf("Invalid kCode Flag")
	}
	if m.KHeaderFlag != "header" {
		return fmt.Errorf("Invalid kHeader Flag")
	}
	if m.KStateFlag != "state" {
		return fmt.Errorf("Invalid kState Flag")
	}

	return nil
}

// To Users

type deployUserMessagesV2 struct {
	UserMessages   []deployUserMessageV2
	CreatorMessage []byte
}

type deployUserMessageV2 struct {
	Cert []byte
	Keys []byte // Encrypted keysToUserV2
}

type deployCreatorMessageV2 struct {
	SkCFlag string
	SkC     []byte
}

func (m *deployCreatorMessageV2) Validate() error {
	if m.SkCFlag != "skC" {
		return fmt.Errorf("Invalid skC Flag")
	}

	return nil
}

type deployuserKeysV2 struct {
	PkC         []byte
	PkCFlag     string
	KHeader     []byte
	KHeaderFlag string
	KCode       []byte
	KCodeFlag   string
	KState      []byte
	KStateFlag  string
}

func (m *deployuserKeysV2) Validate() error {
	if m.PkCFlag != "pkC" {
		return fmt.Errorf("Invalid pkC Flag")
	}
	if m.KCodeFlag != "code" {
		return fmt.Errorf("Invalid kCode Flag")
	}
	if m.KHeaderFlag != "header" {
		return fmt.Errorf("Invalid kHeader Flag")
	}
	if m.KStateFlag != "state" {
		return fmt.Errorf("Invalid kState Flag")
	}

	return nil
}

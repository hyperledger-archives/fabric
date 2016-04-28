package crypto

import (
	"fmt"
)

// Payload

type ePayloadV2 struct {
	Flag       string
	Payload    []byte
	TxCertHash []byte
	TxSign     []byte
}

func (m *ePayloadV2) Validate() error {
	if m.Flag != "payload" {
		return fmt.Errorf("Invalid payload Flag")
	}

	return nil
}

// To Validators

type eValidatorMessagesV2 struct {
	KInvokeFlag string
	KInvoke     []byte
}

func (m *eValidatorMessagesV2) Validate() error {
	if m.KInvokeFlag != "kI" {
		return fmt.Errorf("Invalid kI Flag")
	}

	return nil
}

// To Users

type eUserMessagesV2 struct {
	InvokerMessage []byte
}

type eInvokerMessageV2 struct {
	KInvokeFlag string
	KInvoke     []byte
}

func (m *eInvokerMessageV2) Validate() error {
	if m.KInvokeFlag != "kI" {
		return fmt.Errorf("Invalid kI Flag")
	}

	return nil
}

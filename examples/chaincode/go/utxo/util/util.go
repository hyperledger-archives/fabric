package util

import (
	"bytes"
	"encoding/binary"
)

func decodeInt32(input []byte) (int32, []byte, error) {
	var myint32 int32
	buf1 := bytes.NewBuffer(input[0:4])
	binary.Read(buf1, binary.LittleEndian, &myint32)
	return myint32, input, nil
}

func ReadVarInt(buffer *bytes.Buffer) uint64 {
	var finalResult uint64

	var variableLenInt uint8
	binary.Read(buffer, binary.LittleEndian, &variableLenInt)
	if variableLenInt < 253 {
		finalResult = uint64(variableLenInt)
	} else if variableLenInt == 253 {
		var result uint16
		binary.Read(buffer, binary.LittleEndian, &result)
		finalResult = uint64(result)
	} else if variableLenInt == 254 {
		var result uint32
		binary.Read(buffer, binary.LittleEndian, &result)
		finalResult = uint64(result)
	} else if variableLenInt == 255 {
		var result uint64
		binary.Read(buffer, binary.LittleEndian, &result)
		finalResult = result
	}

	return finalResult
}

func ParseUTXOBytes(txAsUTXOBytes []byte) *TX {
	buffer := bytes.NewBuffer(txAsUTXOBytes)
	var version int32
	binary.Read(buffer, binary.LittleEndian, &version)

	inputCount := ReadVarInt(buffer)

	newTX := &TX{}

	for i := 0; i < int(inputCount); i++ {
		newTXIN := &TX_TXIN{}

		newTXIN.SourceHash = buffer.Next(32)

		binary.Read(buffer, binary.LittleEndian, &newTXIN.Ix)

		// Parse the script length and script bytes
		scriptLength := ReadVarInt(buffer)
		newTXIN.Script = buffer.Next(int(scriptLength))

		// Appears to not be used currently
		binary.Read(buffer, binary.LittleEndian, &newTXIN.Sequence)

		newTX.Txin = append(newTX.Txin, newTXIN)

	}

	// Now the outputs
	outputCount := ReadVarInt(buffer)

	for i := 0; i < int(outputCount); i++ {
		newTXOUT := &TX_TXOUT{}

		binary.Read(buffer, binary.LittleEndian, &newTXOUT.Value)

		// Parse the script length and script bytes
		scriptLength := ReadVarInt(buffer)
		newTXOUT.Script = buffer.Next(int(scriptLength))

		newTX.Txout = append(newTX.Txout, newTXOUT)

	}

	binary.Read(buffer, binary.LittleEndian, &newTX.LockTime)

	return newTX
}

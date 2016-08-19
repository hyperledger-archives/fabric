/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

type largeRowsetChaincode struct {
}

func (lrc *largeRowsetChaincode) retInAdd(ok bool, err error) ([]byte, error) {
	if err != nil {
		return nil, fmt.Errorf("operation failed. %s", err)
	}
	if !ok {
		return nil, errors.New("operation failed. Row with given key already exists")
	}
	return nil, nil
}

// Init called for initializing the chaincode
func (lrc *largeRowsetChaincode) Init(db *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	// Create a new table
	if err := db.CreateTable("Investor", []*shim.ColumnDefinition{
		{Name: "Key", Type: shim.ColumnDefinition_STRING, Key: true},
		{"Name", shim.ColumnDefinition_STRING, false},
		{"Bank", shim.ColumnDefinition_STRING, false},
	}); err != nil {
		//just assume the table exists and was populated
		return nil, nil
	}

	//don't change this... unit test case depends on this
	totalRows := 250
	for i := 0; i < totalRows; i++ {
		col1 := fmt.Sprintf("Key_%d", i)
		col2 := fmt.Sprintf("Name_%d", i)
		col3 := fmt.Sprintf("Bank_%d", i)
		if _, err := lrc.retInAdd(db.InsertRow("Investor", shim.Row{Columns: []*shim.Column{
			&shim.Column{Value: &shim.Column_String_{String_: col1}},
			&shim.Column{Value: &shim.Column_String_{String_: col2}},
			&shim.Column{Value: &shim.Column_String_{String_: col3}},
		}})); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// Run callback representing the invocation of a chaincode
func (lrc *largeRowsetChaincode) Invoke(db *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil, nil
}

// Query callback representing the query of a chaincode.
// If function is "old", will use GetRows (inefficient)
// Otherwise, will use GetRows2 where caller should class close
func (lrc *largeRowsetChaincode) Query(db *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	model := "Investor"

	//if we call GetRows2, call closeIter
	var closeIter func()

	var rowChannel <-chan shim.Row
	var err error
	if function == "" || function == "old" {
		//method 1, not efficient for large row set
		// 1. Call GetRows()
		rowChannel, err = db.GetRows(model, []shim.Column{})
		if err != nil {
			return nil, err
		}
	} else {
		//method 2, more efficient but we have close call close
		// 1. Call GetRows2()
		closeIter, rowChannel, err = db.GetRows2(model, []shim.Column{})
		if err != nil {
			return nil, err
		}
	}

	// 2. Fetch all the rows
	var rows []*shim.Row
	for {
		select {
		case row, ok := <-rowChannel:
			if !ok {
				rowChannel = nil
			} else {
				rows = append(rows, &row)
			}
		}

		if rowChannel == nil {
			break
		}
	}

	// 4. make sure to close iterator if necessary
	if closeIter != nil {
		closeIter()
	}

	// 5. Query some other functions immediately which access to KVS
	col1 := shim.Column{Value: &shim.Column_String_{String_: "Key_2"}}
	_, err = db.GetRow(model, []shim.Column{col1})
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("%d", len(rows))), nil
}

func main() {
	err := shim.Start(new(largeRowsetChaincode))
	if err != nil {
		fmt.Printf("Error starting the chaincode: %s", err)
	}
}

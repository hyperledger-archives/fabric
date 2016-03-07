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

package shim

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

// Logger for the shim package.
var chaincodeLogger = logging.MustGetLogger("chaincode")

// Handler to shim that handles all control logic.
var handler *Handler

// Chaincode is the standard chaincode callback interface that the chaincode developer needs to implement.
type Chaincode interface {
	// Run method will be called during init and for every transaction
	Run(stub *ChaincodeStub, function string, args []string) ([]byte, error)
	// Query is to be used for read-only access to chaincode state
	Query(stub *ChaincodeStub, function string, args []string) ([]byte, error)
}

// ChaincodeStub for shim side handling.
type ChaincodeStub struct {
	UUID string
}

// Peer address derived from command line or env var
var peerAddress string

// Start entry point for chaincodes bootstrap.
func Start(cc Chaincode) error {
	viper.SetEnvPrefix("OPENCHAIN")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	flag.StringVar(&peerAddress, "peer.address", "", "peer address")

	flag.Parse()

	chaincodeLogger.Debug("Peer address: %s", getPeerAddress())

	// Establish connection with validating peer
	clientConn, err := newPeerClientConnection()
	if err != nil {
		chaincodeLogger.Error(fmt.Sprintf("Error trying to connect to local peer: %s", err))
		return fmt.Errorf("Error trying to connect to local peer: %s", err)
	}

	chaincodeLogger.Debug("os.Args returns: %s", os.Args)

	chaincodeSupportClient := pb.NewChaincodeSupportClient(clientConn)

	err = chatWithPeer(chaincodeSupportClient, cc)

	return err
}

func getPeerAddress() string {
	if peerAddress != "" {
		return peerAddress
	}

	if peerAddress = viper.GetString("peer.address"); peerAddress == "" {
		// Assume docker container, return well known docker host address
		peerAddress = "172.17.42.1:30303"
	}

	return peerAddress
}

func newPeerClientConnection() (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if viper.GetBool("peer.tls.enabled") {
		var sn string
		if viper.GetString("peer.tls.server-host-override") != "" {
			sn = viper.GetString("peer.tls.server-host-override")
		}
		var creds credentials.TransportAuthenticator
		if viper.GetString("peer.tls.cert.file") != "" {
			var err error
			creds, err = credentials.NewClientTLSFromFile(viper.GetString("peer.tls.cert.file"), sn)
			if err != nil {
				grpclog.Fatalf("Failed to create TLS credentials %v", err)
			}
		} else {
			creds = credentials.NewClientTLSFromCert(nil, sn)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}
	opts = append(opts, grpc.WithTimeout(1*time.Second))
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithInsecure())
	conn, err := grpc.Dial(getPeerAddress(), opts...)
	if err != nil {
		return nil, err
	}
	return conn, err
}

func chatWithPeer(chaincodeSupportClient pb.ChaincodeSupportClient, cc Chaincode) error {

	// Establish stream with validating peer
	stream, err := chaincodeSupportClient.Register(context.Background())
	if err != nil {
		return fmt.Errorf("Error chatting with leader at address=%s:  %s", getPeerAddress(), err)
	}

	// Create the chaincode stub which will be passed to the chaincode
	//stub := &ChaincodeStub{}

	// Create the shim handler responsible for all control logic
	handler = newChaincodeHandler(getPeerAddress(), stream, cc)

	defer stream.CloseSend()
	// Send the ChaincodeID during register.
	chaincodeID := &pb.ChaincodeID{Name: viper.GetString("chaincode.id.name")}
	payload, err := proto.Marshal(chaincodeID)
	if err != nil {
		return fmt.Errorf("Error marshalling chaincodeID during chaincode registration: %s", err)
	}
	// Register on the stream
	chaincodeLogger.Debug("Registering.. sending %s", pb.ChaincodeMessage_REGISTER)
	handler.serialSend(&pb.ChaincodeMessage{Type: pb.ChaincodeMessage_REGISTER, Payload: payload})
	waitc := make(chan struct{})
	go func() {
		defer close(waitc)
		msgAvail := make(chan *pb.ChaincodeMessage)
		var nsInfo *nextStateInfo
		var in *pb.ChaincodeMessage
		recv := true
		for {
			in = nil
			err = nil
			nsInfo = nil
			if recv {
				recv = false
				go func() {
					var in2 *pb.ChaincodeMessage
					in2, err = stream.Recv()
					msgAvail <- in2
				}()
			}
			select {
			case in = <-msgAvail:
				if err == io.EOF {
					chaincodeLogger.Debug("Received EOF, ending chaincode stream, %s", err)
					return
				} else if err != nil {
					chaincodeLogger.Error(fmt.Sprintf("Received error from server: %s, ending chaincode stream", err))
					return
				} else if in == nil {
					err = fmt.Errorf("Received nil message, ending chaincode stream")
					chaincodeLogger.Debug("Received nil message, ending chaincode stream")
					return
				}
				chaincodeLogger.Debug("[%s]Received message %s from shim", shortuuid(in.Uuid), in.Type.String())
				recv = true
			case nsInfo = <-handler.nextState:
				in = nsInfo.msg
				if in == nil {
					panic("nil msg")
				}
				chaincodeLogger.Debug("[%s]Move state message %s", shortuuid(in.Uuid), in.Type.String())
			}

			// Call FSM.handleMessage()
			err = handler.handleMessage(in)
			if err != nil {
				err = fmt.Errorf("Error handling message: %s", err)
				return
			}
			if nsInfo != nil && nsInfo.sendToCC {
				chaincodeLogger.Debug("[%s]send state message %s", shortuuid(in.Uuid), in.Type.String())
				if err = handler.serialSend(in); err != nil {
					err = fmt.Errorf("Error sending %s: %s", in.Type.String(), err)
					return
				}
			}
		}
	}()
	<-waitc
	return err
}

// GetState function can be invoked by a chaincode to get a state from the ledger.
func (stub *ChaincodeStub) GetState(key string) ([]byte, error) {
	return handler.handleGetState(key, stub.UUID)
}

// PutState function can be invoked by a chaincode to put state into the ledger.
func (stub *ChaincodeStub) PutState(key string, value []byte) error {
	return handler.handlePutState(key, value, stub.UUID)
}

// DelState function can be invoked by a chaincode to delete state from the ledger.
func (stub *ChaincodeStub) DelState(key string) error {
	return handler.handleDelState(key, stub.UUID)
}

// StateRangeQueryIterator allows a chaincode to iterate over a range of
// key/value pairs in the state.
type StateRangeQueryIterator struct {
	handler    *Handler
	uuid       string
	response   *pb.RangeQueryStateResponse
	currentLoc int
}

// RangeQueryState function can be invoked by a chaincode to query of a range
// of keys in the state. Assuming the startKey and endKey are in lexical order,
// an iterator will be returned that can be used to iterate over all keys
// between the startKey and endKey, inclusive. The order in which keys are
// returned by the iterator is random.
func (stub *ChaincodeStub) RangeQueryState(startKey, endKey string) (*StateRangeQueryIterator, error) {
	response, err := handler.handleRangeQueryState(startKey, endKey, stub.UUID)
	if err != nil {
		return nil, err
	}
	return &StateRangeQueryIterator{handler, stub.UUID, response, 0}, nil
}

// HasNext returns true if the range query iterator contains additional keys
// and values.
func (iter *StateRangeQueryIterator) HasNext() bool {
	if iter.currentLoc < len(iter.response.KeysAndValues) || iter.response.HasMore {
		return true
	}
	return false
}

// Next returns the next key and value in the range query iterator.
func (iter *StateRangeQueryIterator) Next() (string, []byte, error) {
	if iter.currentLoc < len(iter.response.KeysAndValues) {
		keyValue := iter.response.KeysAndValues[iter.currentLoc]
		iter.currentLoc++
		return keyValue.Key, keyValue.Value, nil
	} else if !iter.response.HasMore {
		return "", nil, errors.New("No such key")
	} else {
		response, err := iter.handler.handleRangeQueryStateNext(iter.response.ID, iter.uuid)

		if err != nil {
			return "", nil, err
		}

		iter.currentLoc = 0
		iter.response = response
		keyValue := iter.response.KeysAndValues[iter.currentLoc]
		iter.currentLoc++
		return keyValue.Key, keyValue.Value, nil

	}
}

// Close closes the range query iterator. This should be called when done
// reading from the iterator to free up resources.
func (iter *StateRangeQueryIterator) Close() error {
	_, err := iter.handler.handleRangeQueryStateClose(iter.response.ID, iter.uuid)
	return err
}

// InvokeChaincode function can be invoked by a chaincode to execute another chaincode.
func (stub *ChaincodeStub) InvokeChaincode(chaincodeName string, function string, args []string) ([]byte, error) {
	return handler.handleInvokeChaincode(chaincodeName, function, args, stub.UUID)
}

// QueryChaincode function can be invoked by a chaincode to query another chaincode.
func (stub *ChaincodeStub) QueryChaincode(chaincodeName string, function string, args []string) ([]byte, error) {
	return handler.handleQueryChaincode(chaincodeName, function, args, stub.UUID)
}

// TABLE FUNCTIONALITY
// TODO More comments here with documentation

// Table Errors
var (
	// ErrTableNotFound if the specified table cannot be found
	ErrTableNotFound = errors.New("chaincode: Table not found")
)

// CreateTable creates a new table given the table name and column definitions
func (stub *ChaincodeStub) CreateTable(name string, columnDefinitions []*ColumnDefinition) error {

	_, err := stub.getTable(name)
	if err == nil {
		return fmt.Errorf("CreateTable operation failed. Table %s already exists.", name)
	}
	if err != ErrTableNotFound {
		return fmt.Errorf("CreateTable operation failed. %s", err)
	}

	if columnDefinitions == nil || len(columnDefinitions) == 0 {
		return errors.New("Invalid column definitions. Tables must contain at least one column.")
	}

	hasKey := false
	nameMap := make(map[string]bool)
	for i, definition := range columnDefinitions {

		// Check name
		if definition == nil {
			return fmt.Errorf("Column definition %d is invalid. Definition must not be nil.", i)
		}
		if len(definition.Name) == 0 {
			return fmt.Errorf("Column definition %d is invalid. Name must be 1 or more characters.", i)
		}
		if _, exists := nameMap[definition.Name]; exists {
			return fmt.Errorf("Invalid table. Table contains duplicate column name '%s'.", definition.Name)
		}
		nameMap[definition.Name] = true

		// Check type
		switch definition.Type {
		case ColumnDefinition_STRING:
		case ColumnDefinition_INT32:
		case ColumnDefinition_INT64:
		case ColumnDefinition_UINT32:
		case ColumnDefinition_UINT64:
		case ColumnDefinition_BYTES:
		case ColumnDefinition_BOOL:
		default:
			return fmt.Errorf("Column definition %s does not have a valid type.", definition.Name)
		}

		if definition.Key {
			hasKey = true
		}
	}

	if !hasKey {
		return errors.New("Inavlid table. One or more columns must be a key.")
	}

	table := &Table{name, columnDefinitions}
	tableBytes, err := proto.Marshal(table)
	if err != nil {
		return fmt.Errorf("Error marshalling table: %s", err)
	}
	tableNameKey, err := getTableNameKey(name)
	if err != nil {
		return fmt.Errorf("Error creating table key: %s", err)
	}
	err = stub.PutState(tableNameKey, tableBytes)
	if err != nil {
		return fmt.Errorf("Error inserting table in state: %s", err)
	}
	return nil
}

// GetTable returns the table for the specified table name or ErrTableNotFound
// if the table does not exist.
func (stub *ChaincodeStub) GetTable(tableName string) (*Table, error) {
	return stub.getTable(tableName)
}

// DeleteTable deletes and entire table and all associated row
func (stub *ChaincodeStub) DeleteTable(tableName string) error {
	tableNameKey, err := getTableNameKey(tableName)
	if err != nil {
		return err
	}

	// Delete rows
	iter, err := stub.RangeQueryState(tableNameKey+"1", tableNameKey+":")
	if err != nil {
		return fmt.Errorf("Error deleting table: %s", err)
	}
	defer iter.Close()
	for iter.HasNext() {
		key, _, err := iter.Next()
		if err != nil {
			return fmt.Errorf("Error deleting table: %s", err)
		}
		err = stub.DelState(key)
		if err != nil {
			return fmt.Errorf("Error deleting table: %s", err)
		}
	}

	return stub.DelState(tableNameKey)
}

// InsertRow inserts a new row into the specified table.
// Returns -
// true and no error if the row is successfully inserted.
// false and no error if a row already exists for the given key.
// false and a TableNotFoundError if the specified table name does not exist.
// false and an error if there is an unexpected error condition.
func (stub *ChaincodeStub) InsertRow(tableName string, row Row) (bool, error) {
	return stub.insertRowInternal(tableName, row, false)
}

// ReplaceRow updates the row in the specified table.
// Returns -
// true and no error if the row is successfully updated.
// false and no error if a row does not exist the given key.
// flase and a TableNotFoundError if the specified table name does not exist.
// false and an error if there is an unexpected error condition.
func (stub *ChaincodeStub) ReplaceRow(tableName string, row Row) (bool, error) {
	return stub.insertRowInternal(tableName, row, true)
}

// GetRow fetches a row from the specified table for the given key.
func (stub *ChaincodeStub) GetRow(tableName string, key []Column) (Row, error) {

	var row Row

	keyString, err := buildKeyString(tableName, key)
	if err != nil {
		return row, err
	}

	rowBytes, err := stub.GetState(keyString)
	if err != nil {
		return row, fmt.Errorf("Error fetching row from DB: %s", err)
	}

	err = proto.Unmarshal(rowBytes, &row)
	if err != nil {
		return row, fmt.Errorf("Error unmarshalling row: %s", err)
	}

	return row, nil

}

// GetRows returns multiple rows based on a partial key. For example, given table
// | A | B | C | D |
// where A, C and D are keys, GetRows can be called with [A, C] to return
// all rows that have A, C and any value for D as their key. GetRows could
// also be called with A only to return all rows that have A and any value
// for C and D as their key.
func (stub *ChaincodeStub) GetRows(tableName string, key []Column) (<-chan Row, error) {

	keyString, err := buildKeyString(tableName, key)
	if err != nil {
		return nil, err
	}

	iter, err := stub.RangeQueryState(keyString+"1", keyString+":")
	if err != nil {
		return nil, fmt.Errorf("Error fetching rows: %s", err)
	}
	defer iter.Close()

	rows := make(chan Row)

	go func() {
		for iter.HasNext() {
			_, rowBytes, err := iter.Next()
			if err != nil {
				close(rows)
			}

			var row Row
			err = proto.Unmarshal(rowBytes, &row)
			if err != nil {
				close(rows)
			}

			rows <- row

		}
		close(rows)
	}()

	return rows, nil

}

// DeleteRow deletes the row for the given key from the specified table.
func (stub *ChaincodeStub) DeleteRow(tableName string, key []Column) error {

	keyString, err := buildKeyString(tableName, key)
	if err != nil {
		return err
	}

	err = stub.DelState(keyString)
	if err != nil {
		return fmt.Errorf("DeleteRow operation error. Error deleting row: %s", err)
	}

	return nil
}

func (stub *ChaincodeStub) getTable(tableName string) (*Table, error) {

	tableName, err := getTableNameKey(tableName)
	if err != nil {
		return nil, err
	}

	tableBytes, err := stub.GetState(tableName)
	if tableBytes == nil {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("Error fetching table: %s", err)
	}
	table := &Table{}
	err = proto.Unmarshal(tableBytes, table)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling table: %s", err)
	}

	return table, nil
}

func validateTableName(name string) error {
	if len(name) == 0 {
		return errors.New("Inavlid table name. Table name must be 1 or more characters.")
	}

	return nil
}

func getTableNameKey(name string) (string, error) {
	err := validateTableName(name)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(len(name)) + name, nil
}

func buildKeyString(tableName string, keys []Column) (string, error) {

	var keyBuffer bytes.Buffer

	tableNameKey, err := getTableNameKey(tableName)
	if err != nil {
		return "", err
	}

	keyBuffer.WriteString(tableNameKey)

	for _, key := range keys {

		var keyString string
		switch key.Value.(type) {
		case *Column_String_:
			keyString = key.GetString_()
		case *Column_Int32:
			// b := make([]byte, 4)
			// binary.LittleEndian.PutUint32(b, uint32(key.GetInt32()))
			// keyBuffer.Write(b)
			keyString = strconv.FormatInt(int64(key.GetInt32()), 10)
		case *Column_Int64:
			keyString = strconv.FormatInt(key.GetInt64(), 10)
		case *Column_Uint32:
			keyString = strconv.FormatUint(uint64(key.GetInt32()), 10)
		case *Column_Uint64:
			keyString = strconv.FormatUint(key.GetUint64(), 10)
		case *Column_Bytes:
			keyString = string(key.GetBytes())
		case *Column_Bool:
			keyString = strconv.FormatBool(key.GetBool())
		}

		keyBuffer.WriteString(strconv.Itoa(len(keyString)))
		keyBuffer.WriteString(keyString)
	}

	return keyBuffer.String(), nil
}

func getKeyAndVerifyRow(table Table, row Row) ([]Column, error) {

	var keys []Column

	if row.Columns == nil || len(row.Columns) != len(table.ColumnDefinitions) {
		return keys, fmt.Errorf("Table '%s' defines %d columns, but row has %d columns.",
			table.Name, len(table.ColumnDefinitions), len(row.Columns))
	}

	for i, column := range row.Columns {

		// Check types
		var expectedType bool
		switch column.Value.(type) {
		case *Column_String_:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_STRING
		case *Column_Int32:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_INT32
		case *Column_Int64:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_INT64
		case *Column_Uint32:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_UINT32
		case *Column_Uint64:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_UINT64
		case *Column_Bytes:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_BYTES
		case *Column_Bool:
			expectedType = table.ColumnDefinitions[i].Type == ColumnDefinition_BOOL
		default:
			expectedType = false
		}
		if !expectedType {
			return keys, fmt.Errorf("The type for table '%s', column '%s' is '%s', but the column in the row does not match.",
				table.Name, table.ColumnDefinitions[i].Name, table.ColumnDefinitions[i].Type)
		}

		if table.ColumnDefinitions[i].Key {
			keys = append(keys, *column)
		}

	}

	return keys, nil
}

func (stub *ChaincodeStub) isRowPrsent(tableName string, key []Column) (bool, error) {
	keyString, err := buildKeyString(tableName, key)
	if err != nil {
		return false, err
	}
	rowBytes, err := stub.GetState(keyString)
	if err != nil {
		return false, fmt.Errorf("Error fetching row for key %s: %s", keyString, err)
	}
	if rowBytes != nil {
		return true, nil
	}
	return false, nil
}

// insertRowInternal inserts a new row into the specified table.
// Returns -
// true and no error if the row is successfully inserted.
// false and no error if a row already exists for the given key.
// flase and a TableNotFoundError if the specified table name does not exist.
// false and an error if there is an unexpected error condition.
func (stub *ChaincodeStub) insertRowInternal(tableName string, row Row, update bool) (bool, error) {

	table, err := stub.getTable(tableName)
	if err != nil {
		return false, err
	}

	key, err := getKeyAndVerifyRow(*table, row)
	if err != nil {
		return false, err
	}

	present, err := stub.isRowPrsent(tableName, key)
	if err != nil {
		return false, err
	}
	if (present && !update) || (!present && update) {
		return false, nil
	}

	rowBytes, err := proto.Marshal(&row)
	if err != nil {
		return false, fmt.Errorf("Error marshalling row: %s", err)
	}

	keyString, err := buildKeyString(tableName, key)
	if err != nil {
		return false, err
	}
	err = stub.PutState(keyString, rowBytes)
	if err != nil {
		return false, fmt.Errorf("Error inserting row in table %s: %s", tableName, err)
	}

	return true, nil
}

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

package obcca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"time"

	// Needed to use sqlite3
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/sha3"
)

// CA is the base certificate authority.
//
type CA struct {
	db   *sql.DB
	priv *ecdsa.PrivateKey
	cert *x509.Certificate
}

// NewCA sets up a new CA.
//
func NewCA(name string) *CA {
	id := name + "-root"

	if _, err := os.Stat(RootPath); err != nil {
		Info.Println("new CA, creating database, key pair, and certificate")

		if err := os.Mkdir(RootPath, 0755); err != nil {
			Panic.Panicln(err)
		}
	}

	ca := new(CA)

	// open/create DB
	db, err := sql.Open("sqlite3", RootPath+"/"+name+".db")
	if err != nil {
		Panic.Panicln(err)
	}

	if err := db.Ping(); err != nil {
		Panic.Panicln(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Certificates (row INTEGER PRIMARY KEY, id VARCHAR(64), timestamp INTEGER, cert BLOB, hash BLOB)"); err != nil {
		Panic.Panicln(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Users (row INTEGER PRIMARY KEY, id VARCHAR(64), password VARCHAR(64))"); err != nil {
		Panic.Panicln(err)
	}
	ca.db = db

	// open/create signing key
	priv, err := ca.readCAPrivateKey(RootPath, name+".sign")
	if err != nil {
		priv = ca.newCAPrivateKey(RootPath, name+".sign")
	}
	ca.priv = priv

	// open/create self-signed CA certificate
	raw, err := ca.readCertificate(id)
	if err != nil {
		raw, err = ca.newCertificate(id, &ca.priv.PublicKey, time.Now().UnixNano())
		if err != nil {
			Panic.Panicln(err)
		}
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		Panic.Panicln(err)
	}
	ca.cert = cert

	return ca
}

// Close closes down the CA.
//
func (ca *CA) Close() {
	ca.db.Close()
}

func (ca *CA) newCAPrivateKey(RootPath, name string) *ecdsa.PrivateKey {
	curve := elliptic.P384()

	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err == nil {
		raw, err := x509.MarshalECPrivateKey(priv)
		cooked := pem.EncodeToMemory(
			&pem.Block{
				Type:  "ECDSA PRIVATE KEY",
				Bytes: raw,
			})
		if err == nil {
			err = ioutil.WriteFile(RootPath+"/"+name, cooked, 0644)
		}
	}
	if err != nil {
		Panic.Panicln(err)
	}

	return priv
}

func (ca *CA) readCAPrivateKey(RootPath, name string) (*ecdsa.PrivateKey, error) {
	cooked, err := ioutil.ReadFile(RootPath + "/" + name)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(cooked)

	return x509.ParseECPrivateKey(block.Bytes)
}

func (ca *CA) newCertificate(id string, pub *ecdsa.PublicKey, timestamp int64, opt ...pkix.Extension) ([]byte, error) {
	Trace.Println("creating certificate for " + id)

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour * 24 * 90)
	isCA := ca.cert == nil

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "OBC",
			Organization: []string{"IBM"},
			Country:      []string{"US"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		SubjectKeyId:       []byte{1, 2, 3, 4},
		SignatureAlgorithm: x509.ECDSAWithSHA384,
		KeyUsage:           x509.KeyUsageDigitalSignature,

		BasicConstraintsValid: true,
		IsCA: isCA,
	}
	if len(opt) > 0 {
		tmpl.Extensions = opt
		tmpl.ExtraExtensions = opt
	}

	parent := ca.cert
	if isCA {
		parent = &tmpl
	}

	raw, err := x509.CreateCertificate(
		rand.Reader,
		&tmpl,
		parent,
		pub,
		ca.priv,
	)
	if isCA && err != nil {
		Panic.Panicln(err)
	}

	hash := sha3.New384()
	hash.Write(raw)
	if _, err = ca.db.Exec("INSERT INTO Certificates (id, timestamp, cert, hash) VALUES (?, ?, ?, ?)", id, timestamp, raw, hash.Sum(nil)); err != nil {
		if isCA {
			Panic.Panicln(err)
		} else {
			Error.Println(err)
		}
	}

	return raw, err
}

func (ca *CA) readCertificate(id string, opt ...int64) ([]byte, error) {
	Trace.Println("reading certificate for " + id)

	var raw []byte
	var err error
	if len(opt) > 0 && opt[0] != 0 {
		row := ca.db.QueryRow("SELECT cert FROM Certificates WHERE id=? AND timestamp=?", id, opt[0])
		err = row.Scan(&raw)
	} else {
		row := ca.db.QueryRow("SELECT cert FROM Certificates WHERE id=?", id)
		err = row.Scan(&raw)
	}

	return raw, err
}

func (ca *CA) readCertificateByHash(hash []byte) ([]byte, error) {
	Trace.Println("reading certificate for hash " + string(hash))

	var raw []byte
	row := ca.db.QueryRow("SELECT cert FROM Certificates WHERE hash=?", hash)
	err := row.Scan(&raw)

	return raw, err
}

func (ca *CA) readCertificateSet(id string, timestamp int64, num int) ([][]byte, error) {
	Trace.Println("reading certificate for "+id+" / %d", timestamp)

	var raw []byte
	rows, err := ca.db.Query("SELECT cert FROM Certificates WHERE id=? AND timestamp=?", id, timestamp)
	if err != nil {
		return nil, err
	}

	var set [][]byte
	for rows.Next() {
		err := rows.Scan(&raw)
		if err != nil {
			return set, err
		}
		set = append(set, raw)
	}

	return set, err
}

func (ca *CA) newUser(id string, opt ...string) (string, error) {
	Trace.Println("registering user " + id)

	pw, err := ca.readPassword(id)
	if err != nil {
		if len(opt) > 0 && len(opt[0]) > 0 {
			pw = opt[0]
		} else {
			pw = randomString(12)
		}

		_, err = ca.db.Exec("INSERT INTO Users (id, password) VALUES (?, ?)", id, pw)
		if err != nil {
			Error.Println(err)

		}
	}

	return pw, err
}

func (ca *CA) readPassword(id string) (string, error) {
	Trace.Println("reading password for " + id)

	var pw string
	row := ca.db.QueryRow("SELECT password FROM Users WHERE id=?", id)
	err := row.Scan(&pw)

	return pw, err
}

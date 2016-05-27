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

package ca

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/crypto/primitives"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	_ "github.com/mattn/go-sqlite3" // TODO: justify this blank import or remove
)

// CA is the base certificate authority.
type CA struct {
	db *sql.DB

	path string

	priv *ecdsa.PrivateKey
	cert *x509.Certificate
	raw  []byte
}

// CertificateSpec defines the parameter used to create a new certificate.
type CertificateSpec struct {
	id           string
	commonName   string
	serialNumber *big.Int
	pub          interface{}
	usage        x509.KeyUsage
	NotBefore    *time.Time
	NotAfter     *time.Time
	ext          *[]pkix.Extension
}

// AffiliationGroup struct
type AffiliationGroup struct {
	name     string
	parentID int64
	parent   *AffiliationGroup
	preKey   []byte
}

// NewCertificateSpec creates a new certificate spec
//
func NewCertificateSpec(id string, commonName string, serialNumber *big.Int, pub interface{}, usage x509.KeyUsage, notBefore *time.Time, notAfter *time.Time, opt ...pkix.Extension) *CertificateSpec {
	spec := new(CertificateSpec)
	spec.id = id
	spec.commonName = commonName
	spec.serialNumber = serialNumber
	spec.pub = pub
	spec.usage = usage
	spec.NotBefore = notBefore
	spec.NotAfter = notAfter
	spec.ext = &opt
	return spec
}

// NewDefaultPeriodCertificateSpec creates a new certificate spec with notBefore a minute ago and not after 90 days from notBefore.
//
func NewDefaultPeriodCertificateSpec(id string, serialNumber *big.Int, pub interface{}, usage x509.KeyUsage, opt ...pkix.Extension) *CertificateSpec {
	return NewDefaultPeriodCertificateSpecWithCommonName(id, id, serialNumber, pub, usage, opt...)
}

// NewDefaultPeriodCertificateSpecWithCommonName creates a new certificate spec with notBefore a minute ago and not after 90 days from notBefore and a specifc commonName.
//
func NewDefaultPeriodCertificateSpecWithCommonName(id string, commonName string, serialNumber *big.Int, pub interface{}, usage x509.KeyUsage, opt ...pkix.Extension) *CertificateSpec {
	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := notBefore.Add(time.Hour * 24 * 90)
	return NewCertificateSpec(id, commonName, serialNumber, pub, usage, &notBefore, &notAfter, opt...)
}

// NewDefaultCertificateSpec creates a new certificate spec with serialNumber = 1, notBefore a minute ago and not after 90 days from notBefore.
//
func NewDefaultCertificateSpec(id string, pub interface{}, usage x509.KeyUsage, opt ...pkix.Extension) *CertificateSpec {
	serialNumber := big.NewInt(1)
	return NewDefaultPeriodCertificateSpec(id, serialNumber, pub, usage, opt...)
}

// NewDefaultCertificateSpecWithCommonName creates a new certificate spec with serialNumber = 1, notBefore a minute ago and not after 90 days from notBefore and a specific commonName.
//
func NewDefaultCertificateSpecWithCommonName(id string, commonName string, pub interface{}, usage x509.KeyUsage, opt ...pkix.Extension) *CertificateSpec {
	serialNumber := big.NewInt(1)
	return NewDefaultPeriodCertificateSpecWithCommonName(id, commonName, serialNumber, pub, usage, opt...)
}

// GetID returns the spec's ID field/value
//
func (spec *CertificateSpec) GetID() string {
	return spec.id
}

// GetCommonName returns the spec's Common Name field/value
//
func (spec *CertificateSpec) GetCommonName() string {
	return spec.commonName
}

// GetSerialNumber returns the spec's Serial Number field/value
//
func (spec *CertificateSpec) GetSerialNumber() *big.Int {
	return spec.serialNumber
}

// GetPublicKey returns the spec's Public Key field/value
//
func (spec *CertificateSpec) GetPublicKey() interface{} {
	return spec.pub
}

// GetUsage returns the spec's usage (which is the x509.KeyUsage) field/value
//
func (spec *CertificateSpec) GetUsage() x509.KeyUsage {
	return spec.usage
}

// GetNotBefore returns the spec NotBefore (time.Time) field/value
//
func (spec *CertificateSpec) GetNotBefore() *time.Time {
	return spec.NotBefore
}

// GetNotAfter returns the spec NotAfter (time.Time) field/value
//
func (spec *CertificateSpec) GetNotAfter() *time.Time {
	return spec.NotAfter
}

// GetOrganization returns the spec's Organization field/value
//
func (spec *CertificateSpec) GetOrganization() string {
	return GetConfigString("pki.ca.subject.organization")
}

// GetCountry returns the spec's Country field/value
//
func (spec *CertificateSpec) GetCountry() string {
	return GetConfigString("pki.ca.subject.country")
}

// GetSubjectKeyID returns the spec's subject KeyID
//
func (spec *CertificateSpec) GetSubjectKeyID() *[]byte {
	return &[]byte{1, 2, 3, 4}
}

// GetSignatureAlgorithm returns the X509.SignatureAlgorithm field/value
//
func (spec *CertificateSpec) GetSignatureAlgorithm() x509.SignatureAlgorithm {
	return x509.ECDSAWithSHA384
}

// GetExtensions returns the sepc's extensions
//
func (spec *CertificateSpec) GetExtensions() *[]pkix.Extension {
	return spec.ext
}

// NewCA sets up a new CA.
func NewCA(name string) *CA {
	ca := new(CA)
	ca.path = GetConfigString("server.rootpath") + "/" + GetConfigString("server.cadir")

	if _, err := os.Stat(ca.path); err != nil {
		Info.Println("Fresh start; creating databases, key pairs, and certificates.")

		if err := os.MkdirAll(ca.path, 0755); err != nil {
			Panic.Panicln(err)
		}
	}

	// open or create certificate database
	db, err := sql.Open("sqlite3", ca.path+"/"+name+".db")
	if err != nil {
		Panic.Panicln(err)
	}

	if err := db.Ping(); err != nil {
		Panic.Panicln(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Certificates (row INTEGER PRIMARY KEY, id VARCHAR(64), timestamp INTEGER, usage INTEGER, cert BLOB, hash BLOB, kdfkey BLOB)"); err != nil {
		Panic.Panicln(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Users (row INTEGER PRIMARY KEY, id VARCHAR(64), enrollmentId VARCHAR(100), role INTEGER, token BLOB, state INTEGER, key BLOB)"); err != nil {
		Panic.Panicln(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS AffiliationGroups (row INTEGER PRIMARY KEY, name VARCHAR(64), parent INTEGER, FOREIGN KEY(parent) REFERENCES AffiliationGroups(row))"); err != nil {
		Panic.Panicln(err)
	}
	ca.db = db

	// read or create signing key pair
	priv, err := ca.readCAPrivateKey(name)
	if err != nil {
		priv = ca.createCAKeyPair(name)
	}
	ca.priv = priv

	// read CA certificate, or create a self-signed CA certificate
	raw, err := ca.readCACertificate(name)
	if err != nil {
		raw = ca.createCACertificate(name, &ca.priv.PublicKey)
	}
	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		Panic.Panicln(err)
	}

	ca.raw = raw
	ca.cert = cert

	return ca
}

// Close closes down the CA.
func (ca *CA) Close() {
	ca.db.Close()
}

func (ca *CA) createCAKeyPair(name string) *ecdsa.PrivateKey {
	Trace.Println("Creating CA key pair.")

	curve := primitives.GetDefaultCurve()

	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err == nil {
		raw, _ := x509.MarshalECPrivateKey(priv)
		cooked := pem.EncodeToMemory(
			&pem.Block{
				Type:  "ECDSA PRIVATE KEY",
				Bytes: raw,
			})
		err := ioutil.WriteFile(ca.path+"/"+name+".priv", cooked, 0644)
		if err != nil {
			Panic.Panicln(err)
		}

		raw, _ = x509.MarshalPKIXPublicKey(&priv.PublicKey)
		cooked = pem.EncodeToMemory(
			&pem.Block{
				Type:  "ECDSA PUBLIC KEY",
				Bytes: raw,
			})
		err = ioutil.WriteFile(ca.path+"/"+name+".pub", cooked, 0644)
		if err != nil {
			Panic.Panicln(err)
		}
	}
	if err != nil {
		Panic.Panicln(err)
	}

	return priv
}

func (ca *CA) readCAPrivateKey(name string) (*ecdsa.PrivateKey, error) {
	Trace.Println("Reading CA private key.")

	cooked, err := ioutil.ReadFile(ca.path + "/" + name + ".priv")
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(cooked)
	return x509.ParseECPrivateKey(block.Bytes)
}

func (ca *CA) createCACertificate(name string, pub *ecdsa.PublicKey) []byte {
	Trace.Println("Creating CA certificate.")

	raw, err := ca.newCertificate(name, pub, x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign, nil)
	if err != nil {
		Panic.Panicln(err)
	}

	cooked := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: raw,
		})
	err = ioutil.WriteFile(ca.path+"/"+name+".cert", cooked, 0644)
	if err != nil {
		Panic.Panicln(err)
	}

	return raw
}

func (ca *CA) readCACertificate(name string) ([]byte, error) {
	Trace.Println("Reading CA certificate.")

	cooked, err := ioutil.ReadFile(ca.path + "/" + name + ".cert")
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(cooked)
	return block.Bytes, nil
}

func (ca *CA) createCertificate(id string, pub interface{}, usage x509.KeyUsage, timestamp int64, kdfKey []byte, opt ...pkix.Extension) ([]byte, error) {
	spec := NewDefaultCertificateSpec(id, pub, usage, opt...)
	return ca.createCertificateFromSpec(spec, timestamp, kdfKey)
}

func (ca *CA) createCertificateFromSpec(spec *CertificateSpec, timestamp int64, kdfKey []byte) ([]byte, error) {
	Trace.Println("Creating certificate for " + spec.GetID() + ".")

	raw, err := ca.newCertificateFromSpec(spec)
	if err != nil {
		Error.Println(err)
		return nil, err
	}

	hash := primitives.NewHash()
	hash.Write(raw)
	if _, err = ca.db.Exec("INSERT INTO Certificates (id, timestamp, usage, cert, hash, kdfkey) VALUES (?, ?, ?, ?, ?, ?)", spec.GetID(), timestamp, spec.GetUsage(), raw, hash.Sum(nil), kdfKey); err != nil {
		Error.Println(err)
	}

	return raw, err
}

func (ca *CA) newCertificate(id string, pub interface{}, usage x509.KeyUsage, ext []pkix.Extension) ([]byte, error) {
	spec := NewDefaultCertificateSpec(id, pub, usage, ext...)
	return ca.newCertificateFromSpec(spec)
}

func (ca *CA) newCertificateFromSpec(spec *CertificateSpec) ([]byte, error) {
	notBefore := spec.GetNotBefore()
	notAfter := spec.GetNotAfter()

	parent := ca.cert
	isCA := parent == nil

	tmpl := x509.Certificate{
		SerialNumber: spec.GetSerialNumber(),
		Subject: pkix.Name{
			CommonName:   spec.GetCommonName(),
			Organization: []string{spec.GetOrganization()},
			Country:      []string{spec.GetCountry()},
		},
		NotBefore: *notBefore,
		NotAfter:  *notAfter,

		SubjectKeyId:       *spec.GetSubjectKeyID(),
		SignatureAlgorithm: spec.GetSignatureAlgorithm(),
		KeyUsage:           spec.GetUsage(),

		BasicConstraintsValid: true,
		IsCA: isCA,
	}

	if len(*spec.GetExtensions()) > 0 {
		tmpl.Extensions = *spec.GetExtensions()
		tmpl.ExtraExtensions = *spec.GetExtensions()
	}
	if isCA {
		parent = &tmpl
	}

	raw, err := x509.CreateCertificate(
		rand.Reader,
		&tmpl,
		parent,
		spec.GetPublicKey(),
		ca.priv,
	)
	if isCA && err != nil {
		Panic.Panicln(err)
	}

	return raw, err
}

func (ca *CA) readCertificate(id string, usage x509.KeyUsage) ([]byte, error) {
	Trace.Println("Reading certificate for " + id + ".")

	var raw []byte
	err := ca.db.QueryRow("SELECT cert FROM Certificates WHERE id=? AND usage=?", id, usage).Scan(&raw)

	return raw, err
}

func (ca *CA) readCertificate1(id string, ts int64) ([]byte, error) {
	Trace.Println("Reading certificate for " + id + ".")

	var raw []byte
	err := ca.db.QueryRow("SELECT cert FROM Certificates WHERE id=? AND timestamp=?", id, ts).Scan(&raw)

	return raw, err
}

func (ca *CA) readCertificates(id string, opt ...int64) (*sql.Rows, error) {
	Trace.Println("Reading certificatess for " + id + ".")

	if len(opt) > 0 && opt[0] != 0 {
		return ca.db.Query("SELECT cert, kdfkey FROM Certificates WHERE id=? AND timestamp=? ORDER BY usage", id, opt[0])
	}

	return ca.db.Query("SELECT cert, kdfkey FROM Certificates WHERE id=?", id)
}

func (ca *CA) readCertificateSets(id string, start, end int64) (*sql.Rows, error) {
	Trace.Println("Reading certificate sets for " + id + ".")

	return ca.db.Query("SELECT cert, kdfKey, timestamp FROM Certificates WHERE id=? AND timestamp BETWEEN ? AND ? ORDER BY timestamp", id, start, end)
}

func (ca *CA) readCertificateByHash(hash []byte) ([]byte, error) {
	Trace.Println("Reading certificate for hash " + string(hash) + ".")

	var raw []byte
	row := ca.db.QueryRow("SELECT cert FROM Certificates WHERE hash=?", hash)
	err := row.Scan(&raw)

	return raw, err
}

func (ca *CA) isValidAffiliation(affiliation string) (bool, error) {
	Trace.Println("Validating affiliation: " + affiliation)

	var count int
	var err error
	err = ca.db.QueryRow("SELECT count(row) FROM AffiliationGroups WHERE name=?", affiliation).Scan(&count)
	if err != nil {
		Trace.Println("Affiliation <" + affiliation + "> is INVALID.")

		return false, err
	}
	Trace.Println("Affiliation <" + affiliation + "> is VALID.")

	return count == 1, nil
}

//
// Determine if affiliation is required for a given registration request.
//
// Affiliation is required if the role is client or peer.
// Affiliation is not required if the role is validator or auditor.
// 1: client, 2: peer, 4: validator, 8: auditor
//

func (ca *CA) requireAffiliation(role pb.Role) bool {
	roleStr, _ := MemberRoleToString(role)
	Trace.Println("Assigned role is: " + roleStr + ".")

	return role != pb.Role_VALIDATOR && role != pb.Role_AUDITOR
}

// validateAndGenerateEnrollID validates the affiliation subject
func (ca *CA) validateAndGenerateEnrollID(id, affiliation, affiliationRole string, role pb.Role) (string, error) {
	roleStr, _ := MemberRoleToString(role)
	Trace.Println("Validating and generating enrollID for user id: " + id + ", affiliation: " + affiliation + ", affiliationRole: " + affiliationRole + ", role: " + roleStr + ".")

	// Check whether the affiliation is required for the current user.
	//
	// Affiliation is required if the role is client or peer.
	// Affiliation is not required if the role is validator or auditor.
	if ca.requireAffiliation(role) {
		valid, err := ca.isValidAffiliation(affiliation)
		if err != nil {
			return "", err
		}

		if !valid {
			Trace.Println("Invalid affiliation group: ")
			return "", errors.New("Invalid affiliation group " + affiliation)
		}

		return ca.generateEnrollID(id, affiliationRole, affiliation)
	}

	return "", nil
}

// registerUser registers a new member with the CA
//
func (ca *CA) registerUser(id, affiliation, affiliationRole string, role pb.Role, opt ...string) (string, error) {
	roleStr, _ := MemberRoleToString(role)
	Trace.Println("Received request to register user with id: " + id + ", affiliation: " + affiliation + ", affiliationRole: " + affiliationRole + ", role: " + roleStr + ".")

	var tok string
	var err error
	var enrollID string
	enrollID, err = ca.validateAndGenerateEnrollID(id, affiliation, affiliationRole, role)

	if err != nil {
		return "", err
	}
	tok, err = ca.registerUserWithErollID(id, enrollID, role, opt...)
	if err != nil {
		return "", err
	}
	return tok, nil
}

// registerUserWithEnrollID registers a new user and its enrollmentID, role and state
//
func (ca *CA) registerUserWithErollID(id string, enrollID string, role pb.Role, opt ...string) (string, error) {
	roleStr, _ := MemberRoleToString(role)
	Trace.Println("Registering user " + id + " as " + roleStr + ".")

	var row int
	err := ca.db.QueryRow("SELECT row FROM Users WHERE id=?", id).Scan(&row)
	if err == nil {
		return "", errors.New("user is already registered")
	}

	var tok string
	if len(opt) > 0 && len(opt[0]) > 0 {
		tok = opt[0]
	} else {
		tok = randomString(12)
	}

	_, err = ca.db.Exec("INSERT INTO Users (id, enrollmentId, token, role, state) VALUES (?, ?, ?, ?, ?)", id, enrollID, tok, role, 0)

	if err != nil {
		Error.Println(err)
	}

	return tok, err

}

// registerAffiliationGroup registers a new affiliation group
//
func (ca *CA) registerAffiliationGroup(name string, parentName string) error {
	Trace.Println("Registering affiliation group " + name + " parent " + parentName + ".")

	var parentID int
	var err error
	var count int
	err = ca.db.QueryRow("SELECT count(row) FROM AffiliationGroups WHERE name=?", name).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("Affiliation group is already registered")
	}

	if strings.Compare(parentName, "") != 0 {
		err = ca.db.QueryRow("SELECT row FROM AffiliationGroups WHERE name=?", parentName).Scan(&parentID)
		if err != nil {
			return err
		}
	}

	_, err = ca.db.Exec("INSERT INTO AffiliationGroups (name, parent) VALUES (?, ?)", name, parentID)

	if err != nil {
		Error.Println(err)
	}

	return err

}

// deleteUser deletes a user given a name
//
func (ca *CA) deleteUser(id string) error {
	Trace.Println("Deleting user " + id + ".")

	var row int
	err := ca.db.QueryRow("SELECT row FROM Users WHERE id=?", id).Scan(&row)
	if err == nil {
		_, err = ca.db.Exec("DELETE FROM Certificates Where id=?", id)
		if err != nil {
			Error.Println(err)
		}

		_, err = ca.db.Exec("DELETE FROM Users WHERE row=?", row)
		if err != nil {
			Error.Println(err)
		}
	}

	return err
}

// readUser reads a token given an id
//
func (ca *CA) readUser(id string) *sql.Row {
	Trace.Println("Reading token for " + id + ".")

	return ca.db.QueryRow("SELECT role, token, state, key, enrollmentId FROM Users WHERE id=?", id)
}

// readUsers reads users of a given Role
//
func (ca *CA) readUsers(role int) (*sql.Rows, error) {
	Trace.Println("Reading users matching role " + strconv.FormatInt(int64(role), 2) + ".")

	return ca.db.Query("SELECT id, role FROM Users WHERE role&?!=0", role)
}

// readRole returns the user Role given a user id
//
func (ca *CA) readRole(id string) int {
	Trace.Println("Reading role for " + id + ".")

	var role int
	ca.db.QueryRow("SELECT role FROM Users WHERE id=?", id).Scan(&role)

	return role
}

func (ca *CA) readAffiliationGroups() ([]*AffiliationGroup, error) {
	Trace.Println("Reading affilition groups.")

	rows, err := ca.db.Query("SELECT row, name, parent FROM AffiliationGroups")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make(map[int64]*AffiliationGroup)

	for rows.Next() {
		group := new(AffiliationGroup)
		var id int64
		if e := rows.Scan(&id, &group.name, &group.parentID); e != nil {
			return nil, err
		}
		groups[id] = group
	}

	groupList := make([]*AffiliationGroup, len(groups))
	idx := 0
	for _, eachGroup := range groups {
		eachGroup.parent = groups[eachGroup.parentID]
		groupList[idx] = eachGroup
		idx++
	}

	return groupList, nil
}

func (ca *CA) generateEnrollID(id string, role string, affiliation string) (string, error) {
	if id == "" || role == "" || affiliation == "" {
		return "", errors.New("Please provide all the input parameters, id, role and affiliation")
	}

	if strings.Contains(id, "\\") || strings.Contains(role, "\\") || strings.Contains(affiliation, "\\") {
		return "", errors.New("Do not include the escape character \\ as part of the values")
	}

	return id + "\\" + affiliation + "\\" + role, nil
}

func (ca *CA) parseEnrollID(enrollID string) (id string, role string, affiliation string, err error) {

	if enrollID == "" {
		return "", "", "", errors.New("Input parameter missing")
	}

	enrollIDSections := strings.Split(enrollID, "\\")

	if len(enrollIDSections) != 3 {
		return "", "", "", errors.New("Either the userId, Role or affiliation is missing from the enrollmentID")
	}

	id = enrollIDSections[0]
	role = enrollIDSections[2]
	affiliation = enrollIDSections[1]
	err = nil
	return
}

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

package ca

import (
	"time"	
	"strings"
	"bytes"
	"errors"
	"math/big"
	"encoding/asn1"

	"crypto/ecdsa"
 	"crypto/x509"
	"crypto/x509/pkix"

 	"database/sql"
 	
  	"golang.org/x/net/context"
  	"github.com/spf13/viper"
	"github.com/golang/protobuf/proto"
  	"google.golang.org/grpc"

  	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"github.com/hyperledger/fabric/core/crypto/utils"
	
	"google/protobuf"
)

var (
	ACAAttribute = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 10}

)
// ACA is the attribute certificate authority.
type ACA struct {
	*CA
}

// ACAP serves the public GRPC interface of the ACA.
//
type ACAP struct {
	aca *ACA
}

// ACAA serves the administrator GRPC interface of the ACA.
//
type ACAA struct {
	aca *ACA
}

func IsAttributeOID(oid asn1.ObjectIdentifier) bool {
	l := len(oid)
	if len(ACAAttribute) != l {
		return false
	}
	for i := 0; i < l-1; i++ {
		if ACAAttribute[i] != oid[i] {
			return false
		}
	}

	return ACAAttribute[l-1] < oid[l-1]
} 

func initializeACATables(db *sql.DB) error { 
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Attributes (row INTEGER PRIMARY KEY, id VARCHAR(64), affiliation VARCHAR(64), attributeKey VARCHAR(64), validFrom DATETIME, validTo DATETIME,  attributeValue BLOB)"); err != nil {
		return err
	}
	return nil
}

type AttributeOwner struct {
	id string
	affiliation string
}

type AttributePair struct { 
	owner *AttributeOwner
	attributeKey string
	attributeValue []byte
	validFrom time.Time
	validTo time.Time
}

func NewAttributePair(attribute_vals []string, attrOwner *AttributeOwner) (*AttributePair, error) { 
	if len(attribute_vals) < 6 { 
		return nil, errors.New("Invalid attribute entry")
	}
	var attrPair = *new(AttributePair)
	if attrOwner != nil { 
		attrPair.SetOwner(attrOwner)
	} else { 
		attrPair.SetOwner(&AttributeOwner{strings.TrimSpace(attribute_vals[0]), strings.TrimSpace(attribute_vals[1])})
	}
	attrPair.SetAttributeKey(strings.TrimSpace(attribute_vals[2]))
	attrPair.SetAttributeValue([]byte(strings.TrimSpace(attribute_vals[3])))
	//Reading validFrom date
	dateStr := strings.TrimSpace(attribute_vals[4]) 
	if dateStr  != "" { 
			if t,err:=time.Parse(time.RFC3339, dateStr); err != nil { 
				return nil, err
			} else { 
				attrPair.SetValidFrom(t)
			}
	}
	//Reading validTo date
	dateStr = strings.TrimSpace(attribute_vals[5]) 
	if dateStr  != "" { 
			if t,err:=time.Parse(time.RFC3339, dateStr); err != nil { 
				return nil, err
			} else { 
				attrPair.SetValidTo(t)
			}
	}
	return &attrPair, nil
}

func (attrOwner *AttributeOwner) GetId() string {
	return attrOwner.id
}

func (attrOwner *AttributeOwner) GetAffiliation() string { 
	return attrOwner.affiliation
}

func (attrPair *AttributePair) GetOwner() *AttributeOwner {
	return attrPair.owner
}

func (attrPair *AttributePair) SetOwner(owner *AttributeOwner) {
	attrPair.owner = owner
}

func (attrPair *AttributePair) GetId() string {
	return attrPair.owner.GetId()
}

func (attrPair *AttributePair) GetAffiliation() string {
	return attrPair.owner.GetAffiliation()
}

func (attrPair *AttributePair) GetAttributeKey() string { 
	return attrPair.attributeKey
}

func (attrPair *AttributePair) SetAttributeKey(key string) { 
	attrPair.attributeKey = key
}

func (attrPair *AttributePair) GetAttributeValue() []byte {
	return attrPair.attributeValue
}

func (attrPair *AttributePair) SetAttributeValue(val []byte) { 
	attrPair.attributeValue = val
}

func (attrPair *AttributePair) IsValidAt(date time.Time) bool {
	return (attrPair.validFrom.Before(date) || attrPair.validFrom.Equal(date)) && (attrPair.validTo.IsZero() || attrPair.validTo.After(date))  
}

func (attrPair *AttributePair) GetValidFrom() time.Time {
	return attrPair.validFrom
}

func (attrPair *AttributePair) SetValidFrom(date time.Time) { 
	attrPair.validFrom = date
}

func (attrPair *AttributePair) GetValidTo() time.Time {
	return attrPair.validTo
}

func (attrPair *AttributePair) SetValidTo(date time.Time) { 
	attrPair.validTo = date
}

func (attrPair *AttributePair) ToACAAttribute() (*pb.ACAAttribute) { 
	var from, to *google_protobuf.Timestamp
	if attrPair.validFrom.IsZero() {
		from = nil
	} else { 
		from = &google_protobuf.Timestamp{Seconds: attrPair.validFrom.Unix(), Nanos: int32(attrPair.validFrom.UnixNano())}
	}
	if attrPair.validTo.IsZero() { 
		to = nil
	} else { 
		to = &google_protobuf.Timestamp{Seconds: attrPair.validTo.Unix(), Nanos: int32(attrPair.validTo.UnixNano())}

	}
	return &pb.ACAAttribute{attrPair.attributeKey, attrPair.attributeValue, from, to}	
}

// NewACA sets up a new ACA.
func NewACA() *ACA {
	aca := &ACA{NewCA("aca", initializeACATables)}

	return aca
}

func (aca *ACA) getECACertificate() (*x509.Certificate, error) { 
	raw, err := aca.readCACertificate("eca")
	if err != nil {
		return nil, err
	}
	return  x509.ParseCertificate(raw)
}

func (aca *ACA) getTCACertificate() (*x509.Certificate, error) { 
	raw, err := aca.readCACertificate("tca")
	if err != nil {
		return nil, err
	}
	return  x509.ParseCertificate(raw)
}


func (aca *ACA) fetchAttributes(id, affiliation string) ([]*AttributePair, error) { 
	// TODO this attributes should be readed from the outside world in place of configuration file.
	attrs := viper.GetStringMapString("aca.attributes")
	attributes := make([]*AttributePair,0)
	for _, flds := range attrs {
		vals := strings.Fields(flds)
		if len(vals) >= 1 {
			val := ""
			for _,eachVal := range vals { 
				val = val + " " + eachVal
			}
			var attrOwner *AttributeOwner
			attribute_vals := strings.Split(val, ";")
			if len(attribute_vals) >= 6 { 
				attrPair, err := NewAttributePair(attribute_vals, attrOwner)
				if err != nil {
					return nil, errors.New("Invalid attribute entry " + val + " " + err.Error() )
				} else { 
					if attrPair.GetId() != id || attrPair.GetAffiliation() != affiliation { 
						continue
					}
					if attrOwner == nil { 
						attrOwner = attrPair.GetOwner()
					}
					attributes = append(attributes, attrPair)
				}
			} else { 
				Error.Printf("Invalid attribute entry '%v'", vals[0])
			}			
		}
	}
	return attributes, nil
}

func (aca *ACA) populateAttributes(attrs []*AttributePair) (error) { 
	tx, dberr := aca.db.Begin(); 
	if dberr != nil { 
		return dberr
	}
	for _, attr := range attrs {
		if err := aca.populateAttribute(attr); err != nil { 
			dberr := tx.Rollback()
			if dberr != nil { 
				return dberr
			}
			return err
		}
	}
	dberr = tx.Commit()
	if dberr != nil {
		return dberr
	}
	return nil
}

func (aca *ACA) populateAttribute(attr *AttributePair) (error)  {
	var count int
	err := aca.db.QueryRow("SELECT count(row) AS cant FROM Attributes WHERE id=? AND affiliation =? AND attributeKey =?", 
		attr.GetId(), attr.GetAffiliation(), attr.GetAttributeKey()).Scan(&count)
	
	if err != nil { 
		return nil;
	}
	
	if count > 0 { 
		_ , err = aca.db.Exec("UPDATE Attributes SET validFrom = ?, validTo = ?,  attributeValue = ? WHERE  id=? AND affiliation =? AND attributeKey =? AND validFrom < ?",
			attr.GetValidFrom(), attr.GetValidTo(), attr.GetAttributeValue(), attr.GetId(), attr.GetAffiliation(), attr.GetAttributeKey(), attr.GetValidFrom())
		if err != nil { 
			return err
		}
	} else { 
		_ , err = aca.db.Exec("INSERT INTO Attributes (validFrom , validTo,  attributeValue, id, affiliation, attributeKey) VALUES (?,?,?,?,?,?)",
			attr.GetValidFrom(), attr.GetValidTo(), attr.GetAttributeValue(), attr.GetId(), attr.GetAffiliation(), attr.GetAttributeKey())	
		if err != nil { 
			return err
		}
	}
	return nil
}

func (aca *ACA) fetchAndPopulateAttributes(id,affiliation string) (error) {
	var attrs []*AttributePair;
	attrs, err := aca.fetchAttributes(id,affiliation)
	if err != nil { 
		return err
	}
	
	err = aca.populateAttributes(attrs)
	if err != nil { 
		return err
	}
	return nil
}

func (aca *ACA) verifyAttribute(owner *AttributeOwner, attributeName string, valueHash []byte) (*AttributePair, error) {
	var count int
	
	err := aca.db.QueryRow("SELECT count(row) AS cant FROM Attributes WHERE id=? AND affiliation =? AND attributeKey =?", 
		owner.GetId(), owner.GetAffiliation(), attributeName).Scan(&count)
	if err != nil { 
		return nil, err
	}
	
	if count == 0 { 
		return nil, nil
	}
	
	var attKey string
	var attValue []byte
	var validFrom, validTo time.Time
	err = aca.db.QueryRow("SELECT attributeKey, attributeValue, validFrom, validTo AS cant FROM Attributes WHERE id=? AND affiliation =? AND attributeKey =?", 
		owner.GetId(), owner.GetAffiliation(),attributeName).Scan(&attKey, &attValue, &validFrom, &validTo)
	if err != nil { 
		return nil, err
	}
	
	hashValue := utils.Hash(attValue)
	if bytes.Compare(hashValue, valueHash) != 0 { 
		return nil, nil
	}
	return &AttributePair{owner,attKey, attValue, validFrom, validTo}, nil
}

// FetchAttributes fetchs the attributes from the outside world and populate them into the database.
func (acap *ACAP) FetchAttributes(ctx context.Context, in *pb.ACAFetchAttrReq) (*pb.ACAFetchAttrResp, error) {
	Trace.Println("grpc ACAP:FetchAttributes")
	cert, err := acap.aca.getECACertificate()
	if err != nil { 
		return &pb.ACAFetchAttrResp{Status: pb.ACAFetchAttrResp_FAILURE}, errors.New("Error getting ECA certificate.")
	}
	
	ecaPub := cert.PublicKey.(*ecdsa.PublicKey)	
	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(in.Signature.R)
	s.UnmarshalText(in.Signature.S)

	in.Signature = nil
	
	hash := utils.NewHash()
	raw, _ := proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(ecaPub, hash.Sum(nil), r, s) == false {
		return &pb.ACAFetchAttrResp{Status: pb.ACAFetchAttrResp_FAILURE}, errors.New("signature does not verify")
	}
	
	cert, err = x509.ParseCertificate(in.ECert.Cert)
	
	if err != nil { 
		return &pb.ACAFetchAttrResp{Status: pb.ACAFetchAttrResp_FAILURE}, err
	}
	var id, affiliation string
	id, _,  affiliation, err = acap.aca.parseEnrollId(cert.Subject.CommonName)
	if err != nil { 
		return &pb.ACAFetchAttrResp{Status: pb.ACAFetchAttrResp_FAILURE}, err
	}
	
	err = acap.aca.fetchAndPopulateAttributes(id,affiliation)
	if err != nil { 
		return &pb.ACAFetchAttrResp{Status: pb.ACAFetchAttrResp_FAILURE}, err
	}
	
	return &pb.ACAFetchAttrResp{Status: pb.ACAFetchAttrResp_SUCCESS}, nil
}

func (acap *ACAP) createRequestAttributeResponse(status pb.ACAAttrResp_StatusCode, cert *pb.Cert) (*pb.ACAAttrResp) {
	resp := &pb.ACAAttrResp{status, cert, nil}
	rawReq, err := proto.Marshal(resp)
	if err != nil {
		return &pb.ACAAttrResp{pb.ACAAttrResp_FAILURE, nil, nil}
	}
	
	
	r, s, err := utils.ECDSASignDirect(acap.aca.priv, rawReq)
	if err != nil {
		return &pb.ACAAttrResp{pb.ACAAttrResp_FAILURE, nil, nil}
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	resp.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}
	
	return resp
}

// RequestAttributes lookups the atributes in the database and return a certificate with attributes included in the request and found in the database.
func (acap *ACAP) RequestAttributes(ctx context.Context, in *pb.ACAAttrReq) (*pb.ACAAttrResp, error) {
	Trace.Println("grpc ACAP:RequestAttributes")
	cert, err := acap.aca.getTCACertificate()
	if err != nil { 
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), errors.New("Error getting TCA certificate.")
	}
	
	tcaPub := cert.PublicKey.(*ecdsa.PublicKey)	
	r, s := big.NewInt(0), big.NewInt(0)
	r.UnmarshalText(in.Signature.R)
	s.UnmarshalText(in.Signature.S)

	in.Signature = nil
	
	hash := utils.NewHash()
	raw, _ := proto.Marshal(in)
	hash.Write(raw)
	if ecdsa.Verify(tcaPub, hash.Sum(nil), r, s) == false {
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), errors.New("signature does not verify")
	}
	
	cert, err = x509.ParseCertificate(in.ECert.Cert)
	
	if err != nil { 
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), err
	}
	var id, affiliation string
	id, _,  affiliation, err = acap.aca.parseEnrollId(cert.Subject.CommonName)
	if err != nil { 
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), err
	}
	//Before continue with the request we perform a refresh of the attributes.
	err = acap.aca.fetchAndPopulateAttributes(id, affiliation)
	if err != nil { 
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), err
	}
	
	var verifyCounter int
	attributes := make([]AttributePair, 0)
	owner := &AttributeOwner{id, affiliation}
	for _, attrPair := range(in.Attributes) {
		verifiedPair, _ := acap.aca.verifyAttribute(owner, attrPair.AttributeName, attrPair.AttributeValueHash)
		if  verifiedPair != nil { 
			verifyCounter++	
			attributes = append(attributes, *verifiedPair)
		}
	}
	
	count := len(in.Attributes)
	if count == 0 { 
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_NO_ATTRIBUTES_FOUND, nil), nil
	}
	
	extensions := make([]pkix.Extension, 0)
	extensions, err = acap.addAttributesToExtensions(&attributes, extensions)
	if err != nil { 
			return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), err
	}
	
	spec := NewDefaultCertificateSpec(id, cert.PublicKey, cert.KeyUsage, extensions...)
	raw, err = acap.aca.newCertificateFromSpec(spec); 
	if err != nil {
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FAILURE,nil), err
	}
	
	if count == verifyCounter { 
		return acap.createRequestAttributeResponse(pb.ACAAttrResp_FULL_SUCCESSFUL, &pb.Cert{raw}), nil 
	}		
	return acap.createRequestAttributeResponse(pb.ACAAttrResp_PARTIAL_SUCCESSFUL, &pb.Cert{raw}), nil 
}

func (acap *ACAP) addAttributesToExtensions(attributes *[]AttributePair, extensions []pkix.Extension) ([]pkix.Extension, error) {
	count := 11
	exts := extensions
	for _, a := range *attributes {
		//Save the position of the attribute extension on the header.
		att := a.ToACAAttribute()
		raw, err := proto.Marshal(att)	
		if err != nil { 
			continue
		}
		exts = append(exts, pkix.Extension{Id: asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, count}, Critical: false, Value: raw})
		count++
	}
	return exts, nil
}

// ReadCACertificate reads the certificate of the ACA.
//
func (acap *ACAP) ReadCACertificate(ctx context.Context, in *pb.Empty) (*pb.Cert, error) {
	Trace.Println("grpc ACAP:ReadCACertificate")

	return &pb.Cert{acap.aca.raw}, nil
}

func (aca *ACA) startACAP(srv *grpc.Server) {
	pb.RegisterACAPServer(srv, &ACAP{aca})
}

//func (aca *ACA) startACAA(srv *grpc.Server) {
//	pb.RegisterACAAServer(srv, &TCAA{tca})
//}

// Start starts the ECA.
//
func (eca *ACA) Start(srv *grpc.Server) {
	eca.startACAP(srv)
	//eca.startACAA(srv)

	Info.Println("ACA started.")
}

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
 "errors"
 "crypto/x509"

 "database/sql"

  pb "github.com/hyperledger/fabric/membersrvc/protos"
  "golang.org/x/net/context"
  "github.com/spf13/viper"

)

// ACA is the attribute certificate authority.
type ACA struct {
	*CA
	eca *ECA
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
		attrPair.SetOwner(&AttributeOwner{attribute_vals[0], attribute_vals[1]})
	}
	attrPair.SetAttributeKey(attribute_vals[2])
	attrPair.SetAttributeValue([]byte(attribute_vals[3]))
	//Reading validFrom date
	if attribute_vals[4] != "" { 
			if t,err:=time.Parse(time.RFC3339, attribute_vals[4]); err != nil { 
				return nil, err
			} else { 
				attrPair.SetValidFrom(t)
			}
	}
	//Reading validTo date
	if attribute_vals[4] != "" { 
			if t,err:=time.Parse(time.RFC3339, attribute_vals[5]); err != nil { 
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

// NewACA sets up a new ACA.
func NewACA() *ACA {
	aca := &ACA{NewCA("aca", initializeACATables)}

	return aca
}

func (aca *ACA) getECACertificate() (*x509.Certificate, error) { 
	raw, err := aca.readCACertificate("eca")
	if err != nil {
		return err
	}
	return  x509.ParseCertificate(raw)
}

func (aca *ACA) fetchAttributes(id, affiliation string) []*AttributePair { 
	// TODO this attributes should be readed from the outside world in place of configuration file.
	attrs := viper.GetStringMapString("aca.attributes")
	attributes := make([]*AttributePair,0)
	for _, flds := range attrs {
		vals := strings.Fields(flds)
		if len(vals) >= 1 {
			var attrOwner *AttributeOwner
			attribute_vals := strings.Split(vals[0], ";")
			if len(attribute_vals) >= 6 { 
				attrPair, err := NewAttributePair(attribute_vals, attrOwner)
				if err != nil {
					Error.Printf("Invalid attribute entry '%v'", vals[0])
				} else { 
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
	return attributes
}

// FetchAttributes fetchs the attributes from the outside world and populate them into the database.
func (acap *ACAP) FetchAttributes(ctx context.Context, in *pb.ACAFetchAttrReq) (*pb.ACAFetchAttrResp, error) {
	Trace.Println("grpc ACAP:FetchAttributes")
	return nil, nil
}

// RequestAttributes lookups the atributes in the database and return a certificate with attributes included in the request and found in the database.
func (acap *ACAP) RequestAttributes(ctx context.Context, in *pb.ACAAttrReq) (*pb.ACAAttrResp, error) {
	Trace.Println("grpc ACAP:RequestAttributes")
	return nil, nil
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

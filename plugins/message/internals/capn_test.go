// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package internals

import (
	"encoding/json"
	"log"
	"testing"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestEncodeDecode(t *testing.T) {
	personJson := `
	{
		"name":	"Tommy",
		"age":	12
	}
	`

	var iface interface{}
	json.Unmarshal([]byte(personJson), &iface)
	var msgEnc InputMsg
	msgEnc.MsgType = 222
	//msgEnc.UID = "333"
	//msgEnc.SetPayLoad(iface)

	bytes, err := msgEnc.Encode()
	if err != nil {
		log.Fatalln("Failed to encode: ", err)
	}

	var msgDec InputMsg
	err = msgDec.Decode(bytes)
	if err != nil {
		log.Fatalln("Failed to decode: ", err)
	}

	if msgEnc.MsgType != msgDec.MsgType {
		log.Fatal("MsgType not matched.")
	}

	// if msgEnc.UID != msgDec.UID {
	// 	log.Fatal("UID not matched.")
	// }

	var person Person
	err = json.Unmarshal(msgDec.Payload, &person)
	if err != nil {
		log.Fatalln("Failed to unmarshal payload: ", err)
	}

	if person.Name != "Tommy" || person.Age != 12 {
		log.Fatalln("Payload not match.")
	}
}

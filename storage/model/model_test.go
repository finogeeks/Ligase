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

package model

import (
	"github.com/finogeeks/ligase/storage/implements/accounts"
	"github.com/finogeeks/ligase/storage/implements/devices"
	"github.com/finogeeks/ligase/storage/implements/encryptoapi"
	"github.com/finogeeks/ligase/storage/implements/keydb"
	"github.com/finogeeks/ligase/storage/implements/presence"
	"github.com/finogeeks/ligase/storage/implements/publicroomapi"
	"github.com/finogeeks/ligase/storage/implements/pushapi"
	"github.com/finogeeks/ligase/storage/implements/roomserver"
	"github.com/finogeeks/ligase/storage/implements/syncapi"
)

func test() {
	a := new(syncapi.Database)
	testSync(a)

	b := new(accounts.Database)
	testAcc(b)

	c := new(devices.Database)
	testDev(c)

	d := new(encryptoapi.Database)
	testEncrypt(d)

	e := new(keydb.Database)
	testKey(e)

	f := new(presence.Database)
	testPresence(f)

	g := new(publicroomapi.Database)
	testPub(g)

	h := new(pushapi.DataBase)
	testPush(h)

	i := new(roomserver.Database)
	testRS(i)

}

func testSync(input SyncAPIDatabase) {
}

func testAcc(input AccountsDatabase) {
}

func testAS(input AppServiceDatabase) {
}

func testDev(input DeviceDatabase) {
}

func testEncrypt(input EncryptorAPIDatabase) {
}

func testKey(input KeyDatabase) {
}

func testPresence(input PresenceDatabase) {
}

func testPub(input PublicRoomAPIDatabase) {
}

func testPush(input PushAPIDatabase) {
}

func testRS(input RoomServerDatabase) {
}

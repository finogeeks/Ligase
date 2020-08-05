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

package common

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/finogeeks/ligase/common/domain"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/common/utils"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"gopkg.in/macaroon.v2"
)

var tokenByteLength = 12 //生成16byte base64 encode string
var LoadDomainFromDB = false

func CalcStringHashCode(str string) uint32 {
	hash := fnv.New32a()
	hash.Write([]byte(str))
	return hash.Sum32()
}

func CalcStringHashCode64(str string) uint64 {
	hash := fnv.New64a()
	hash.Write([]byte(str))
	return hash.Sum64()
}

func BuildToken(key, id, loc, userId, deviceIdentifier string,
	guest bool, deviceID, deviceType string,
	human bool,
) (string, error) {
	mac, err := macaroon.New([]byte(key), []byte(id), loc, macaroon.V1)
	if err != nil {
		return "", err
	}

	mac.AddFirstPartyCaveat([]byte("gen = 1"))
	mac.AddFirstPartyCaveat([]byte("user_id = " + userId))
	mac.AddFirstPartyCaveat([]byte("type = access"))

	random, err := BuildRandomURLEncString()
	if err != nil {
		return "", err
	}
	mac.AddFirstPartyCaveat([]byte("nonce = " + random))
	mac.AddFirstPartyCaveat([]byte("did = " + deviceID))
	mac.AddFirstPartyCaveat([]byte("didType = " + deviceType))
	mac.AddFirstPartyCaveat([]byte("deviceIdentifier = " + deviceIdentifier))
	mac.AddFirstPartyCaveat([]byte("ts = " + strconv.FormatInt(time.Now().Unix(), 10)))
	if human == true {
		mac.AddFirstPartyCaveat([]byte("human = true"))
	} else {
		mac.AddFirstPartyCaveat([]byte("human = false"))
	}

	if guest == true {
		mac.AddFirstPartyCaveat([]byte("guest = true"))
	}
	bytes, err := mac.MarshalBinary()
	res := base64.RawURLEncoding.EncodeToString(bytes)
	// log.Infof("BuildToken token:%s sig:%s\n", res, base64.RawURLEncoding.EncodeToString(mac.Signature()))

	return res, nil
}

func BuildSuperAdminToken(key, id, loc, userId, deviceIdentifier string,
	guest bool, deviceID, deviceType string,
	human bool, ts int64,
) (string, error) {
	mac, err := macaroon.New([]byte(key), []byte(id), loc, macaroon.V1)
	if err != nil {
		return "", err
	}

	mac.AddFirstPartyCaveat([]byte("gen = 1"))
	mac.AddFirstPartyCaveat([]byte("user_id = " + userId))
	mac.AddFirstPartyCaveat([]byte("type = access"))

	random, err := BuildStaticURLEncString()
	if err != nil {
		return "", err
	}
	mac.AddFirstPartyCaveat([]byte("nonce = " + random))
	mac.AddFirstPartyCaveat([]byte("did = " + deviceID))
	mac.AddFirstPartyCaveat([]byte("didType = " + deviceType))
	mac.AddFirstPartyCaveat([]byte("deviceIdentifier = " + deviceIdentifier))
	mac.AddFirstPartyCaveat([]byte("ts = " + strconv.FormatInt(ts, 10)))
	if human == true {
		mac.AddFirstPartyCaveat([]byte("human = true"))
	} else {
		mac.AddFirstPartyCaveat([]byte("human = false"))
	}

	if guest == true {
		mac.AddFirstPartyCaveat([]byte("guest = true"))
	}
	bytes, err := mac.MarshalBinary()
	res := base64.RawURLEncoding.EncodeToString(bytes)
	// log.Infof("BuildStaticToken token:%s sig:%s\n", res, base64.RawURLEncoding.EncodeToString(mac.Signature()))

	return res, nil
}

func ExtractToken(key, id, loc, token string) (*authtypes.Device, bool, error) {
	bytes, err := base64.RawURLEncoding.DecodeString(token)
	guest := false
	if err != nil {
		return nil, guest, err
	}

	mac, err := macaroon.New([]byte(key), []byte(id), loc, macaroon.V1)
	if err != nil {
		return nil, guest, err
	}

	err = mac.UnmarshalBinary(bytes)
	if err != nil {
		return nil, guest, err
	}

	caveats, err := mac.VerifySignature([]byte(key), nil)
	if err != nil {
		return nil, guest, err
	}

	var dev authtypes.Device
	//log.Infof("ExtractToken input_token:%s base64_token:%s sig:%s\n", token, base64.RawURLEncoding.EncodeToString(bytes), base64.RawURLEncoding.EncodeToString(mac.Signature()))
	for _, val := range caveats {
		//log.Infof("ExtractToken caveat:%s loc:%s vid:%s \n", val)
		res := strings.Split(val, " ")
		if res[0] == "user_id" {
			dev.UserID = res[2]
		} else if res[0] == "did" {
			dev.ID = res[2]
		} else if res[0] == "didType" {
			dev.DeviceType = res[2]
		} else if res[0] == "human" {
			dev.IsHuman, _ = strconv.ParseBool(res[2])
		} else if res[0] == "deviceIdentifier" {
			dev.Identifier = res[2]
		} else if res[0] == "guest" {
			guest = true
			dev.IsHuman = true
		}
	}
	return &dev, guest, nil
}

func BuildDevice(idg *uid.UidGenerator, did *string, isHuman, genNewDevice bool) (string, string, error) {
	deviceID := ""
	deviceType := ""
	id, err := idg.Next()
	if err != nil {
		return "", "", err
	}

	if did == nil {
		deviceID = fmt.Sprintf("%s:%d", "virtual", id)
		deviceType = "virtual"
	} else {
		if genNewDevice {
			deviceID = fmt.Sprintf("%s:%d", *did, id)
		} else {
			deviceID = *did
		}

		deviceType = "actual"
	}

	if !isHuman {
		deviceType = "bot"
	}

	return deviceID, deviceType, nil
}

func BuildRandomURLEncString() (string, error) {
	b := make([]byte, tokenByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// url-safe no padding
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func BuildStaticURLEncString() (string, error) {
	b := make([]byte, tokenByteLength)
	b = []byte("super__admin")

	// url-safe no padding
	return base64.RawURLEncoding.EncodeToString(b), nil
}

var DomainFromID = utils.DomainFromID

func CheckValidDomain(id string, domains []string) bool {
	return domain.CheckValidDomain(id, domains)
}

func GetRemoteIP(r *http.Request) string {
	xRealIP := r.Header.Get("X-Real-Ip")
	xForwardedFor := r.Header.Get("X-Forwarded-For")

	// If both empty, return IP from remote address
	if xRealIP == "" && xForwardedFor == "" {
		var remoteIP string

		// If there are colon in remote address, remove the port number
		// otherwise, return remote address as is
		if strings.ContainsRune(r.RemoteAddr, ':') {
			remoteIP, _, _ = net.SplitHostPort(r.RemoteAddr)
		} else {
			remoteIP = r.RemoteAddr
		}

		return remoteIP
	}

	for _, address := range strings.Split(xForwardedFor, ",") {
		return strings.TrimSpace(address)
	}

	// If nothing succeed, return X-Real-IP
	return xRealIP
}

func FilterEventTypes(events *[]gomatrixserverlib.ClientEvent, types *[]string, notTypes *[]string) *[]gomatrixserverlib.ClientEvent {
	delIndex := map[int]bool{}
	updated := false
	for index, ev := range *events {
		if len(*notTypes) > 0 {
			for _, typeString := range *notTypes {
				if ev.Type == typeString {
					delIndex[index] = true
					updated = true
					//*events = append((*events)[:index], (*events)[index+1:]...)
					break
				}
			}
		}

		if len(*types) > 0 {
			del := true
			for _, typeString := range *types {
				if ev.Type == typeString {
					del = false
					break
				}
			}
			if del {
				delIndex[index] = true
				updated = true
			}
		}
	}

	if updated {
		upEvents := []gomatrixserverlib.ClientEvent{}
		for index, ev := range *events {
			if _, ok := delIndex[index]; !ok {
				upEvents = append(upEvents, ev)
			} else {
				log.Infof("filter not interested event:%s roomid:%s", ev.EventID, ev.RoomID)
			}
		}
		return &upEvents
	}

	return events
}

func IsActualDevice(deviceType string) bool {
	if deviceType == "actual" || deviceType == "internal" {
		return true
	}
	return false
}

var DefaultKeyCount = func() map[string]int {
	m := make(map[string]int)
	m["curve25519"] = 50
	m["signed_curve25519"] = 50
	return m
}

var InitKeyCount = func() map[string]int {
	m := make(map[string]int)
	m["curve25519"] = 0
	m["signed_curve25519"] = 0
	return m
}

func GetJsonResponse(data []byte) util.JSONResponse {
	var result util.JSONResponse
	err := json.Unmarshal(data, &result)
	if err != nil {
		log.Errorf("GetJsonResponse Unmarshal error %v", err)
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: jsonerror.Unknown(err.Error()),
		}
	}

	return result
}

func GetDeviceMac(deviceID string) string {
	index := strings.LastIndex(deviceID, ":")
	if index == -1 {
		return deviceID
	}
	return deviceID[0:index]
}

func IsRelatedRequest(key string, instance, total uint32, multiWrite bool) bool {
	if instance == CalcStringHashCode(key)%total {
		return true
	}

	if multiWrite {
		if instance == (CalcStringHashCode(key)+1)%total {
			return true
		}
	}

	return false
}

func IsRelatedSyncRequest(reqInstance, curInstance, total uint32, multiWrite bool) bool {
	if reqInstance == curInstance {
		return true
	}
	if multiWrite {
		if (reqInstance+1)%total == curInstance {
			return true
		}
	}
	return false
}

func GetSyncInstance(key string, total uint32) uint32 {
	return CalcStringHashCode(key) % total
}

var PathExists = utils.PathExists

var GetStringHash = utils.GetStringHash

var DoCompress = utils.DoCompress

var DoUnCompress = utils.DoUnCompress

func UnmarshalJSON(req *http.Request, iface interface{}) error {
	content, _ := ioutil.ReadAll(req.Body)
	if len(content) == 0 {
		return nil
	}

	if err := json.Unmarshal(content, iface); err != nil {
		return err
	}
	return nil
}

func BuildPreBatch(streamPos, timestamp int64) string {
	return fmt.Sprintf("p:%d_t:%d", streamPos, timestamp)
}

func PanicTrace(kb int) []byte {
	s := []byte("/src/runtime/panic.go")
	e := []byte("\ngoroutine ")
	line := []byte("\n")
	stack := make([]byte, kb<<10)
	length := runtime.Stack(stack, false)
	start := bytes.Index(stack, s)
	if start < 0 {
		start = 0
	}
	if length > len(stack) {
		length = len(stack)
	}
	stack = stack[start:length]
	start = bytes.Index(stack, line) + 1
	stack = stack[start:]
	end := bytes.LastIndex(stack, line)
	if end != -1 {
		stack = stack[:end]
	}
	end = bytes.Index(stack, e)
	if end != -1 {
		stack = stack[:end]
	}
	stack = bytes.TrimRight(stack, "\n")
	return stack
}

func LogStack() {
	data := PanicTrace(16)
	log.Info(string(data))
}

func GetMsgType(content map[string]interface{}) (string, bool) {
	value, exists := content["msgtype"]
	if !exists {
		return "", false
	}
	msgtype, ok := value.(string)
	if !ok {
		return "", false
	}
	return msgtype, true
}

func IsMediaEv(content map[string]interface{}) bool {
	msgtype, ok := GetMsgType(content)
	if !ok {
		return false
	}
	return msgtype == "m.image" || msgtype == "m.audio" || msgtype == "m.video" || msgtype == "m.file"
}

func SplitMxc(s string) (domain, netdiskID string) {
	tmpUrl := strings.TrimPrefix(s, "mxc://")
	ss := strings.Split(tmpUrl, "/")
	if len(ss) != 2 {
		return "", s
	} else {
		return ss[0], ss[1]
	}
}

func GetDomainByUserID(userID string) string {
	if userID == "" {
		return userID
	}
	ss := strings.Split(userID,":")
	return ss[len(ss)-1]
}

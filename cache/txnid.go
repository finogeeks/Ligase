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

package cache

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	TXN_DURATION = 30
)

func (rc *RedisCache) GetTxnID(roomID, msgID string) (string, bool) {
	key := fmt.Sprintf("msgid:%s", roomID)
	value, err := rc.HGetString(key, msgID)
	if err != nil {
		return "", false
	}
	s := strings.Split(value, ":")
	if len(s) <= 1 {
		rc.HDel(key, msgID)
		return "", false
	}
	ts, err := strconv.ParseInt(s[0], 10, 64)
	if err != nil {
		rc.HDel(key, msgID)
		return "", false
	}
	if time.Now().Unix()-ts > TXN_DURATION {
		rc.HDel(key, msgID)
		return "", false
	} else {
		return strings.Join(s[1:], ":"), true
	}
}

func (rc *RedisCache) PutTxnID(roomID, txnID, eventID string) error {
	key := fmt.Sprintf("msgid:%s", roomID)
	return rc.HSet(key, txnID, fmt.Sprintf("%d:%s", time.Now().Unix(), eventID))
}

func (rc *RedisCache) ScanTxnID(cursor uint64, count int) ([]string, uint64, error) {
	match := "msgid:*"
	rs, next, err := rc.Scan(cursor, match, count)
	result := []string{}
	for _, r := range rs {
		result = append(result, string(r.([]byte)))
	}
	return result, next, err
}

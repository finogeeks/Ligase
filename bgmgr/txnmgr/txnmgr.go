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

package txnmgr

import (
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
	"strconv"
	"strings"
	"time"
)

type TxnMgr struct {
	cache        service.Cache
	ScanInterval int
}

func NewTxnMgr(
	cache service.Cache,
) *TxnMgr {
	return &TxnMgr{
		cache:        cache,
		ScanInterval: 30000,
	}
}

func (tm *TxnMgr) Start() {
	go func() {
		t := time.NewTicker(time.Millisecond * time.Duration(tm.ScanInterval))
		for {
			select {
			case <-t.C:
				tm.cleanOldTxnID()
			}
		}
	}()
}

func (tm *TxnMgr) cleanOldTxnID() {
	var cursor uint64 = 0
	var count = 1000
	for {
		var result []string
		var err error
		result, cursor, err = tm.cache.ScanTxnID(cursor, count)
		if err != nil {
			log.Errorf("Scan msgid cursor:%d err:%v", cursor, err)
			return
		}
		for _, key := range result {
			tm.cleanExpireTxnID(key)
		}
		if cursor == 0 {
			break
		}
	}
}

func (tm *TxnMgr) cleanExpireTxnID(key string) {
	var cursor uint64 = 0
	var count = 1000
	batch := []string{}
	for {
		var result map[string]interface{}
		var err error
		result, cursor, err = tm.cache.HScan(key, "*", cursor, count)
		if err != nil {
			log.Errorf("HScan msgid cursor:%d err:%v", cursor, err)
			return
		}
		for k, v := range result {
			s := strings.Split(string(v.([]byte)), ":")
			if len(s) <= 1 {
				batch = append(batch, k)
			} else {
				ts, err := strconv.ParseInt(s[0], 10, 64)
				if err != nil || time.Now().Unix()-ts > 30 {
					batch = append(batch, k)
				}
			}
		}
		if cursor == 0 {
			break
		}
	}
	if len(batch) > 0 {
		multi := make([]interface{}, len(batch))
		for i, d := range batch {
			multi[i] = d
		}
		tm.cache.HDelMulti(key, multi)
	}
}

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

package adapter

import (
	"math/rand"
	"reflect"
	"time"
)

const (
	DEBUG_LEVEL_DEBUG = "debug"
	DEBUG_LEVEL_PROD = "prod"
)

type KafkaCommonCfg struct {
	EnableIdempotence bool
	//only avoid kafka sync send has problem can force to async send
	ForceAsyncSend bool
	ReplicaFactor  int
	NumPartitions  int
	NumProducers   int
}

type DomainCfg struct {
	Domains []string
}

type LockCfg struct {
	Timeout int
	Wait    int
	Force   bool
}

type DistLockCfg struct {
	LockInstance     LockCfg
	LockRoomState    LockCfg
	LockRoomStateExt LockCfg
}

type DebugCfg struct {
	DebugLevel string
}

type CommonCfg struct {
	Kafka    KafkaCommonCfg
	Domain   DomainCfg
	DistLock DistLockCfg
	Debug    DebugCfg
	Cache  	 CacheCfg
}

type CacheCfg struct {
	TokenExpire   int64
	UtlExpire int64
	LatestToken int
}

var AdapterCfg CommonCfg

func SetKafkaEnableIdempotence(enableIdempotence bool) {
	AdapterCfg.Kafka.EnableIdempotence = enableIdempotence
}

func GetKafkaEnableIdempotence() bool {
	return AdapterCfg.Kafka.EnableIdempotence
}

func SetKafkaForceAsyncSend(forceAsyncSend bool) {
	AdapterCfg.Kafka.ForceAsyncSend = forceAsyncSend
}

func GetKafkaForceAsyncSend() bool {
	return AdapterCfg.Kafka.ForceAsyncSend
}

func SetKafkaReplicaFactor(replicaFactor int) {
	AdapterCfg.Kafka.ReplicaFactor = replicaFactor
}

func GetKafkaReplicaFactor() int {
	return AdapterCfg.Kafka.ReplicaFactor
}

func SetKafkaNumPartitions(numPartitions int) {
	AdapterCfg.Kafka.NumPartitions = numPartitions
}

func GetKafkaNumPartitions() int {
	return AdapterCfg.Kafka.NumPartitions
}

func SetKafkaNumProducers(numProducers int) {
	AdapterCfg.Kafka.NumProducers = numProducers
}

func GetKafkaNumProducers() int {
	return AdapterCfg.Kafka.NumProducers
}

func SetDomainCfg(domains []string) {
	AdapterCfg.Domain.Domains = domains
}

func GetDomainCfg() []string {
	return AdapterCfg.Domain.Domains
}

func Contains(obj interface{}, target interface{}) bool {
	targetValue := reflect.ValueOf(target)
	switch reflect.TypeOf(target).Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < targetValue.Len(); i++ {
			if targetValue.Index(i).Interface() == obj {
				return true
			}
		}
	case reflect.Map:
		if targetValue.MapIndex(reflect.ValueOf(obj)).IsValid() {
			return true
		}
	}
	return false
}

func CheckSameDomain(domains []string) bool {
	checkDomains := GetDomainCfg()
	for _, domain := range domains {
		if !Contains(domain, checkDomains) {
			return false
		}
	}
	return true
}

func GetDistLockCfg() DistLockCfg {
	return AdapterCfg.DistLock
}

func SetDistLockItemCfg(item string, timeout, wait int, force bool) {
	lockCfg := LockCfg{
		Timeout: timeout,
		Wait:    wait,
		Force:   force,
	}
	switch item {
	case "instance":
		AdapterCfg.DistLock.LockInstance = lockCfg
	case "room_state":
		AdapterCfg.DistLock.LockRoomState = lockCfg
	case "room_state_ext":
		AdapterCfg.DistLock.LockRoomStateExt = lockCfg
	}
}

func SetDebugLevel(debugLevel string) {
	AdapterCfg.Debug.DebugLevel = debugLevel
}

func GetDebugLevel() string {
	return AdapterCfg.Debug.DebugLevel
}

func Random(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

func SetCacheCfg(tokenExpire, utlExpire int64, latestToken int) {
	AdapterCfg.Cache.TokenExpire = tokenExpire
	AdapterCfg.Cache.UtlExpire = utlExpire
	AdapterCfg.Cache.LatestToken = latestToken
}

func GetTokenExpire() int64 {
	return AdapterCfg.Cache.TokenExpire
}

func GetUtlExpire() int64 {
	return AdapterCfg.Cache.UtlExpire
}

func GetLatestToken() int {
	return AdapterCfg.Cache.LatestToken
}

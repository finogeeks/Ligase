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

package selector

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/finogeeks/ligase/core"
)

const consistentWithBoundedLoadName = "consistent-boundedload"

func init() {
	core.RegisterSelector(consistentWithBoundedLoadName, NewConsistent)
}

type ConsistentWithBoundedLoad struct {
	Consistent
	totalLoad int64

	sync.RWMutex
}

func NewConsistentWithBoundedLoad(conf interface{}) (core.ISelector, error) {
	val := &ConsistentWithBoundedLoad{}
	val.keys = map[uint64]string{}
	val.sortedSet = []uint64{}
	val.loadMap = map[string]*int64{}
	return val, nil
}

// It uses Consistent Hashing With Bounded loads
//
// https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html
//
// to pick the least loaded host that can serve the key
//
// It returns ErrNoHosts if the ring has no hosts in it.
//
func (c *ConsistentWithBoundedLoad) GetNode(key string) (string, error) {
	c.RLock()
	defer c.RUnlock()

	if len(c.keys) == 0 {
		return "", ErrNoKeys
	}

	h := c.hash(key)
	idx := c.search(h)

	i := idx
	for {
		host := c.keys[c.sortedSet[i]]
		if c.loadOK(host) {
			return host, nil
		}
		i++
		if i >= len(c.keys) {
			i = 0
		}
	}
}

// Sets the load of `host` to the given `load`
func (c *ConsistentWithBoundedLoad) UpdateLoad(host string, load int64) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.loadMap[host]; !ok {
		return
	}
	c.totalLoad -= *c.loadMap[host]
	c.loadMap[host] = &load
	c.totalLoad += load
}

// Increments the load of host by 1
//
// should only be used with if you obtained a host with GetLeast
func (c *ConsistentWithBoundedLoad) Inc(host string) {
	atomic.AddInt64(c.loadMap[host], 1)
	atomic.AddInt64(&c.totalLoad, 1)
}

// Decrements the load of host by 1
//
// should only be used with if you obtained a host with GetLeast
func (c *ConsistentWithBoundedLoad) Done(host string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.loadMap[host]; !ok {
		return
	}
	atomic.AddInt64(c.loadMap[host], -1)
	atomic.AddInt64(&c.totalLoad, -1)
}

// Deletes host from the ring
func (c *ConsistentWithBoundedLoad) Remove(host string) bool {
	c.Lock()
	defer c.Unlock()

	for i := 0; i < replicationFactor; i++ {
		h := c.hash(fmt.Sprintf("%s%d", host, i))
		delete(c.keys, h)
		c.delSlice(h)
	}
	delete(c.loadMap, host)
	return true
}

// Returns the maximum load of the single host
// which is:
// (total_load/number_of_hosts)*1.25
// total_load = is the total number of active requests served by hosts
// for more info:
// https://research.googleblog.com/2017/04/consistent-hashing-with-bounded-loads.html
func (c *ConsistentWithBoundedLoad) MaxLoad() int64 {
	if c.totalLoad == 0 {
		c.totalLoad = 1
	}
	var avgLoadPerNode float64
	avgLoadPerNode = float64(c.totalLoad / int64(len(c.loadMap)))
	if avgLoadPerNode == 0 {
		avgLoadPerNode = 1
	}
	avgLoadPerNode = math.Ceil(avgLoadPerNode * 1.25)
	return int64(avgLoadPerNode)
}

func (c *ConsistentWithBoundedLoad) loadOK(host string) bool {
	// a safety check if someone performed c.Done more than needed
	if c.totalLoad < 0 {
		c.totalLoad = 0
	}

	var avgLoadPerNode float64
	avgLoadPerNode = float64((c.totalLoad + 1) / int64(len(c.loadMap)))
	if avgLoadPerNode == 0 {
		avgLoadPerNode = 1
	}
	avgLoadPerNode = math.Ceil(avgLoadPerNode * 1.25)

	val, ok := c.loadMap[host]
	if !ok {
		panic(fmt.Sprintf("given host(%s) not in loadsMap", host))
	}

	if float64(*val)+1 <= avgLoadPerNode {
		return true
	}

	return false
}

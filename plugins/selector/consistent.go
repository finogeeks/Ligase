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
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/finogeeks/ligase/core"
	blake2b "github.com/minio/blake2b-simd"
)

const replicationFactor = 10
const consistentName = "consistent"

var ErrNoKeys = errors.New("no keys added")
var defaultLoad = int64(0)

func init() {
	core.RegisterSelector(consistentName, NewConsistent)
}

type Consistent struct {
	keys      map[uint64]string
	sortedSet []uint64
	loadMap   map[string]*int64

	sync.RWMutex
}

func NewConsistent(conf interface{}) (core.ISelector, error) {
	return &Consistent{
		keys:      map[uint64]string{},
		sortedSet: []uint64{},
		loadMap:   map[string]*int64{},
	}, nil
}

func (c *Consistent) GetName() string {
	return consistentName
}

func (c *Consistent) AddNode(host string) {
	c.Lock()
	defer c.Unlock()

	if _, ok := c.loadMap[host]; ok {
		return
	}

	c.loadMap[host] = &defaultLoad
	for i := 0; i < replicationFactor; i++ {
		h := c.hash(fmt.Sprintf("%s%d", host, i))
		c.keys[h] = host
		c.sortedSet = append(c.sortedSet, h)

	}
	// sort hashes ascendingly
	sort.Slice(c.sortedSet, func(i int, j int) bool {
		if c.sortedSet[i] < c.sortedSet[j] {
			return true
		}
		return false
	})
}

// Returns the host that owns `key`.
//
// As described in https://en.wikipedia.org/wiki/Consistent_hashing
//
// It returns ErrNoKeys if the ring has no hosts in it.
func (c *Consistent) GetNode(key string) (string, error) {
	c.RLock()
	defer c.RUnlock()

	if len(c.keys) == 0 {
		return "", ErrNoKeys
	}

	h := c.hash(key)
	idx := c.search(h)
	return c.keys[c.sortedSet[idx]], nil
}

// Deletes host from the ring
func (c *Consistent) DelNode(host string) bool {
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

func (c *Consistent) search(key uint64) int {
	idx := sort.Search(len(c.sortedSet), func(i int) bool {
		return c.sortedSet[i] >= key
	})

	if idx >= len(c.sortedSet) {
		idx = 0
	}
	return idx
}

func (c *Consistent) delSlice(val uint64) {
	for i := 0; i < len(c.sortedSet); i++ {
		if c.sortedSet[i] == val {
			c.sortedSet = append(c.sortedSet[:i], c.sortedSet[i+1:]...)
		}
	}
}

func (c *Consistent) hash(key string) uint64 {
	out := blake2b.Sum512([]byte(key))
	return binary.LittleEndian.Uint64(out[:])
}

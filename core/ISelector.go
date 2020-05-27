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

package core

import (
	"errors"
	"log"
	"sync"
)

type ISelector interface {
	GetName() string
	AddNode(node string)
	GetNode(node string) (string, error)
	DelNode(node string) bool
}

var regSelectorMu sync.RWMutex
var newSelectorHandler = make(map[string]func(conf interface{}) (ISelector, error))
var selectorMap sync.Map

func RegisterSelector(name string, f func(conf interface{}) (ISelector, error)) {
	regSelectorMu.Lock()
	defer regSelectorMu.Unlock()

	log.Printf("ISelector Register: %s func\n", name)
	if f == nil {
		log.Panicf("ISelector Register: %s func nil\n", name)
	}

	if _, ok := newSelectorHandler[name]; ok {
		log.Panicf("ISelector Register: %s already registered\n", name)
	}

	newSelectorHandler[name] = f
}

func GetSelector(name string, conf interface{}) (ISelector, error) {
	f := newSelectorHandler[name]
	if f == nil {
		return nil, errors.New("unknown selector " + name)
	}

	val, ok := selectorMap.Load(name)
	if ok {
		return val.(ISelector), nil
	}

	sel, err := f(conf)
	if err == nil {
		selectorMap.Store(name, sel)
	}

	return sel, err
}

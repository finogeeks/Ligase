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

package lifecycle

import (
	"errors"
)

type Callback func() error

var (
	beforeStartup  = make(map[string]Callback)
	afterStartup   = make(map[string]Callback)
	beforeShutdown = make(map[string]Callback)
	afterShutdown  = make(map[string]Callback)
)

var (
	afterShutdownArr  []Callback
	beforeShutdownArr []Callback
	afterStartupArr   []Callback
	beforeStartupArr  []Callback
)

func BeforeStartup(name string, cb Callback) error {
	if _, ok := beforeStartup[name]; ok {
		return errors.New("lifecycle duplicate register BeforeStartup " + name)
	}
	beforeStartup[name] = cb
	beforeStartupArr = append(beforeStartupArr, cb)
	return nil
}

func AfterStartup(name string, cb Callback) error {
	if _, ok := afterStartup[name]; ok {
		return errors.New("lifecycle duplicate register AfterStartup " + name)
	}
	afterStartup[name] = cb
	afterStartupArr = append(afterStartupArr, cb)
	return nil
}

func BeforeShutdown(name string, cb Callback) error {
	if _, ok := beforeShutdown[name]; ok {
		return errors.New("lifecycle duplicate register BeforeShutdown " + name)
	}
	beforeShutdown[name] = cb
	beforeShutdownArr = append(beforeShutdownArr, cb)
	return nil
}

func AfterShutdown(name string, cb Callback) error {
	if _, ok := afterShutdown[name]; ok {
		return errors.New("lifecycle duplicate register AfterShutdown " + name)
	}
	afterShutdown[name] = cb
	afterShutdownArr = append(afterShutdownArr, cb)
	return nil
}

func RunBeforeStartup() error {
	if beforeStartupArr != nil {
		for _, v := range beforeStartupArr {
			err := v()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RunAfterStartup() error {
	if afterStartupArr != nil {
		for _, v := range afterStartupArr {
			err := v()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RunBeforeShutdown() error {
	if beforeShutdownArr != nil {
		for _, v := range beforeShutdownArr {
			err := v()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func RunAfterShutdown() error {
	if afterStartupArr != nil {
		for _, v := range afterStartupArr {
			err := v()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

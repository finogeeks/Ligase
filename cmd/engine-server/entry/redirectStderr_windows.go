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

package entry

import (
	"log"
	"os"
	"syscall"
)

func initPanicFile(file string) {
	errFile, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file:%v", err)
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setStdHandle := kernel32.NewProc("SetStdHandle")
	sh := syscall.STD_ERROR_HANDLE
	v, _, err := setStdHandle.Call(uintptr(sh), uintptr(errFile.Fd()))
	if v == 0 {
		log.Fatalf("Failed to dup stderr:%v", err)
	}
}

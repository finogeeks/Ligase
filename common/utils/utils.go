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

package utils

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func DomainFromID(id string) (string, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ID %q missing ':'", id)
	}
	return parts[1], nil
}

func NickNameFromID(id string) (string, error) {
	s := strings.Index(id, "@")
	if s < 0 {
		return id, fmt.Errorf("invalid ID %q missing '@'", id)
	}

	s += len("@")
	e := strings.Index(id[s:], ":")
	if e < 0 {
		return id, fmt.Errorf("invalid ID %q missing ':'", id)
	}

	return id[s : s+e], nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func GetStringHash(s string) string {
	signByte := []byte(s)
	hash := md5.New()
	hash.Write(signByte)
	return hex.EncodeToString(hash.Sum(nil))
}

func DoCompress(src []byte) []byte {
	var in bytes.Buffer
	w, _ := zlib.NewWriterLevel(&in, zlib.BestCompression)
	w.Write(src)
	w.Close()
	return in.Bytes()
}

func DoUnCompress(compressSrc []byte) []byte {
	b := bytes.NewReader(compressSrc)
	var out bytes.Buffer
	r, _ := zlib.NewReader(b)
	io.Copy(&out, r)
	return out.Bytes()
}

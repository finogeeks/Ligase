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

package encryption

type Encryption interface {
	Init(e bool, k string, m bool)
	CheckCrypto(eventType string) bool
	CheckMirror(eventType string) bool
	Encrypt(data []byte) []byte
	Decrypt(data []byte) []byte
	DecryptLicense(license string) string
	EncryptDebug(data []byte) []byte
	DecryptDebug(data []byte) []byte
}

type EmptyEncryption struct {
}

// Init init encryption vars
func (*EmptyEncryption) Init(e bool, k string, m bool) {
}

// CheckCrypto check if need to use crypto
func (e *EmptyEncryption) CheckCrypto(eventType string) bool {
	return false
}

// CheckMirror check if need to backup plaintext message events
func (e *EmptyEncryption) CheckMirror(eventType string) bool {
	return false
}

// Encrypt encrypt []byte with key
func (e *EmptyEncryption) Encrypt(data []byte) []byte {
	return data
}

// Decrypt decrypt []byte with key
func (e *EmptyEncryption) Decrypt(data []byte) []byte {
	return data
}

func (e *EmptyEncryption) DecryptLicense(license string) string {
	return license
}

// EncryptDebug encrypt data with key
func (e *EmptyEncryption) EncryptDebug(data []byte) []byte {
	return data
}

// DecryptDebug decrypt data with key
func (e *EmptyEncryption) DecryptDebug(data []byte) []byte {
	return data
}

var impl Encryption = new(EmptyEncryption)

func SetImpl(_impl Encryption) {
	impl = _impl
}

// Init init encryption vars
func Init(e bool, k string, m bool) {
	impl.Init(e, k, m)
}

// CheckCrypto check if need to use crypto
func CheckCrypto(eventType string) bool {
	return impl.CheckCrypto(eventType)
}

// CheckMirror check if need to backup plaintext message events
func CheckMirror(eventType string) bool {
	return impl.CheckMirror(eventType)
}

// Encrypt encrypt []byte with key
func Encrypt(data []byte) []byte {
	return impl.Encrypt(data)
}

// Decrypt decrypt []byte with key
func Decrypt(data []byte) []byte {
	return impl.Decrypt(data)
}

func DecryptLicense(license string) string {
	return impl.DecryptLicense(license)
}

// EncryptDebug encrypt data with key
func EncryptDebug(data []byte) []byte {
	return impl.EncryptDebug(data)
}

// DecryptDebug decrypt data with key
func DecryptDebug(data []byte) []byte {
	return impl.DecryptDebug(data)
}

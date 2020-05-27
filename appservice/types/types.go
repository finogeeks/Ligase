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

package types

import (
	"github.com/finogeeks/ligase/common/config"
	"sync"
)

// ApplicationServiceWorkerState is a type that couples an application service,
// a lockable condition as well as some other state variables, allowing the
// roomserver to notify appservice workers when there are events ready to send
// externally to application services.
type ApplicationServiceWorkerState struct {
	AppService config.ApplicationService
	Cond       *sync.Cond
	// Events ready to be sent
	EventsReady bool
	// Backoff exponent (2^x secs). Max 6, aka 64s.
	Backoff int
}

// NotifyNewEvents wakes up all waiting goroutines, notifying that events remain
// in the event queue for this application service worker.
func (a *ApplicationServiceWorkerState) NotifyNewEvents() {
	a.Cond.L.Lock()
	a.EventsReady = true
	a.Cond.Broadcast()
	a.Cond.L.Unlock()
}

// FinishEventProcessing marks all events of this worker as being sent to the
// application service.
func (a *ApplicationServiceWorkerState) FinishEventProcessing() {
	a.Cond.L.Lock()
	a.EventsReady = false
	a.Cond.L.Unlock()
}

// WaitForNewEvents causes the calling goroutine to wait on the worker state's
// condition for a broadcast or similar wakeup, if there are no events ready.
func (a *ApplicationServiceWorkerState) WaitForNewEvents() {
	a.Cond.L.Lock()
	if !a.EventsReady {
		a.Cond.Wait()
	}
	a.Cond.L.Unlock()
}

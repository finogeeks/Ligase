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

package api

import (
	"context"
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqPostSystemManager{})
}

type ReqPostSystemManager struct{}

func (ReqPostSystemManager) GetRoute() string       { return "/{type}" }
func (ReqPostSystemManager) GetMetricsName() string { return "system_manager" }
func (ReqPostSystemManager) GetMsgType() int32      { return internals.MSG_POST_SYSTEM_MANAGER }
func (ReqPostSystemManager) GetAPIType() int8       { return apiconsumer.APITypeExternal }
func (ReqPostSystemManager) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostSystemManager) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostSystemManager) GetPrefix() []string                  { return []string{"sys"} }
func (ReqPostSystemManager) NewRequest() core.Coder {
	return new(external.PostSystemManagerRequest)
}
func (ReqPostSystemManager) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostSystemManagerRequest)
	if vars != nil {
		msg.Type = vars["type"]
	}
	return nil
}
func (ReqPostSystemManager) NewResponse(code int) core.Coder {
	return nil
}
func (r ReqPostSystemManager) Process(ctx context.Context, ud interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	req := msg.(*external.PostSystemManagerRequest)

	switch req.Type {
	case "gc":
		r.processGc()
	case "free":
		r.processFree()
	}
	return http.StatusOK, nil
}

func (ReqPostSystemManager) processGc() {
	log.Errorf("start process gc from system manager")
	state := new(runtime.MemStats)
	runtime.ReadMemStats(state)
	debug.FreeOSMemory()
	log.Errorf("system manager before gc mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
		state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
	runtime.GC()
	runtime.ReadMemStats(state)
	log.Errorf("system manager after gc mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
		state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
}

func (ReqPostSystemManager) processFree() {
	log.Errorf("start process free os memory from system manager")
	state := new(runtime.MemStats)
	runtime.ReadMemStats(state)
	log.Errorf("system manager before free os memory mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
		state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
	debug.FreeOSMemory()
	runtime.ReadMemStats(state)
	log.Errorf("system manager after free os memory mem: alloc %d, sys %d, alloctime %d, freetime %d, gc %d heapobjects %d, routine %d, cpu %d",
		state.Alloc/1000, state.Sys/1000, state.Mallocs, state.Frees, state.NumGC, state.HeapObjects, runtime.NumGoroutine(), runtime.NumCPU())
}

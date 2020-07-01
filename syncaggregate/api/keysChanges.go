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
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetKeysChanges{})
}

type ReqGetKeysChanges struct{}

func (ReqGetKeysChanges) GetRoute() string       { return "/keys/changes" }
func (ReqGetKeysChanges) GetMetricsName() string { return "look up changes" }
func (ReqGetKeysChanges) GetMsgType() int32      { return internals.MSG_GET_KEYS_CHANGES }
func (ReqGetKeysChanges) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetKeysChanges) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetKeysChanges) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetKeysChanges) GetPrefix() []string                  { return []string{"r0", "unstable"} }
func (ReqGetKeysChanges) NewRequest() core.Coder {
	return new(external.GetKeysChangesRequest)
}
func (ReqGetKeysChanges) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetKeysChangesRequest)
	req.ParseForm()
	query := req.URL.Query()
	msg.From = query.Get("from")
	msg.To = query.Get("to")
	return nil
}
func (ReqGetKeysChanges) NewResponse(code int) core.Coder {
	return new(syncapitypes.KeyChanged)
}
func (r ReqGetKeysChanges) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	if !common.IsRelatedRequest(device.UserID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	req := msg.(*external.GetKeysChangesRequest)

	if !common.IsActualDevice(device.DeviceType) {
		return http.StatusOK, &syncapitypes.KeyChanged{
			Changes: []string{},
		}
	}

	fromStr := req.From
	toStr := req.To
	ctx := context.Background()

	if fromStr == "" || toStr == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("'from' and 'to' must be set in query parameters")
	}

	fromPos, err := r.parseKCFromNextBatch(fromStr)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}
	toPos, err := r.parseKCFromNextBatch(toStr)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	userID := device.UserID

	kcTimeLine := c.keyChangeRepo.GetHistory()
	if kcTimeLine == nil {
		return http.StatusOK, &syncapitypes.KeyChanged{
			Changes: []string{},
		}
	}

	var changedUserIDs []string
	changedUserIDs = []string{}

	kcMap := c.keyChangeRepo.GetHistory()
	if kcMap != nil {
		friendShipMap := c.userTimeLine.GetFriendShip(userID, true)
		if friendShipMap != nil {
			friendShipMap.Range(func(key, _ interface{}) bool {
				if val, ok := kcMap.Load(key.(string)); ok {
					feed := val.(*feedstypes.KeyChangeStream)
					if feed.GetOffset() >= int64(fromPos) && feed.GetOffset() <= int64(toPos) {
						changedUserIDs = append(changedUserIDs, key.(string))
					}
				}
				return true
			})
		}
	}

	return http.StatusOK, &syncapitypes.KeyChanged{
		Changes: changedUserIDs,
	}
}

func (ReqGetKeysChanges) parseKCFromNextBatch(input string) (int, error) {
	offsets := strings.Split(input, "_")
	if len(offsets) == 1 {
		if input != "" {
			return strconv.Atoi(input)
		}
	} else {
		for _, val := range offsets {
			items := strings.Split(val, ":")
			if items[0] == "kc" {
				return strconv.Atoi(items[1])
			}
		}
	}

	return -1, errors.New("can't parse input token")
}

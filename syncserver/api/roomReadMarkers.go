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
	"github.com/finogeeks/ligase/common/jsonerror"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqPostRoomReadMarkers{})
}

type ReqPostRoomReadMarkers struct{}

func (ReqPostRoomReadMarkers) GetRoute() string       { return "/rooms/{roomID}/read_markers" }
func (ReqPostRoomReadMarkers) GetMetricsName() string { return "set_receipt" }
func (ReqPostRoomReadMarkers) GetMsgType() int32      { return internals.MSG_POST_READMARK }
func (ReqPostRoomReadMarkers) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPostRoomReadMarkers) GetMethod() []string {
	return []string{http.MethodPost, http.MethodOptions}
}
func (ReqPostRoomReadMarkers) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPostRoomReadMarkers) GetPrefix() []string                  { return []string{"r0"} }
func (ReqPostRoomReadMarkers) NewRequest() core.Coder {
	return new(external.PostRoomReadMarkersRequest)
}
func (ReqPostRoomReadMarkers) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PostRoomReadMarkersRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.ReceiptType = vars["receiptType"]
	}
	err := common.UnmarshalJSON(req, msg)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPostRoomReadMarkers) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPostRoomReadMarkers) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PostRoomReadMarkersRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}
	data := &types.ReceiptContent{
		RoomID:      req.RoomID,
		UserID:      device.UserID,
		DeviceID:    device.Identifier,
		EventID:     "",
		ReceiptType: req.ReceiptType,
		Source: 	 "remark api",
	}
	c.receiptConsumer.OnReceipt(ctx, data)
	return http.StatusOK, nil
}

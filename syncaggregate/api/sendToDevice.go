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
	"io/ioutil"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/types"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func init() {
	apiconsumer.SetServices("sync_aggregate_api")
	apiconsumer.SetAPIProcessor(ReqPutSendToDevice{})
}

type ReqPutSendToDevice struct{}

func (ReqPutSendToDevice) GetRoute() string       { return "/sendToDevice/{eventType}/{txnId}" }
func (ReqPutSendToDevice) GetMetricsName() string { return "look up changes" }
func (ReqPutSendToDevice) GetMsgType() int32      { return internals.MSG_PUT_SENT_TO_DEVICE }
func (ReqPutSendToDevice) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqPutSendToDevice) GetMethod() []string {
	return []string{http.MethodPut, http.MethodOptions}
}
func (ReqPutSendToDevice) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqPutSendToDevice) GetPrefix() []string                  { return []string{"r0", "inr0", "unstable"} }
func (ReqPutSendToDevice) NewRequest() core.Coder {
	return new(external.PutSendToDeviceRequest)
}
func (ReqPutSendToDevice) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.PutSendToDeviceRequest)
	if vars != nil {
		msg.EventType = vars["eventType"]
		msg.TxnId = vars["txnId"]
	}
	var err error
	msg.Content, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return nil
}
func (ReqPutSendToDevice) NewResponse(code int) core.Coder {
	return nil
}
func (ReqPutSendToDevice) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	resp := []uint32{}
	for i := uint32(0); i < cfg.MultiInstance.Total; i++ {
		resp = append(resp, i)
	}
	return resp
}
func (ReqPutSendToDevice) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.PutSendToDeviceRequest)

	eventType := req.EventType
	sender := device.UserID

	log.Infof("sendToDevice user %s deviceID %s device type %s", device.UserID, device.ID, device.DeviceType)
	if !common.IsActualDevice(device.DeviceType) {
		return http.StatusOK, nil
	}

	stdRq := types.StdRequest{}
	json.Unmarshal(req.Content, &stdRq)

	for uid, deviceMap := range stdRq.Sender {
		if common.IsRelatedRequest(uid, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
			// uid is local domain
			for deviceID, cont := range deviceMap {
				//log.Errorf("start process sendToDevice for user %s, device %s", uid, deviceID)
				dataStream := types.StdEvent{}
				dataStream.Type = eventType
				dataStream.Sender = sender
				dataStream.Content = cont

				// wildcard all devices
				if deviceID == "*" {
					deviceCollection := c.cache.GetDevicesByUserID(uid)
					for _, val := range *deviceCollection {
						if !common.IsActualDevice(val.DeviceType) {
							log.Infof("sendToDevice to all devices for user %s, but device %s is not actual", uid, val.ID)
							continue
						}

						c.stdEventTimeline.AddSTDEventStream(&dataStream, uid, val.ID)
					}
				} else {
					/*dev := cache.GetDeviceByDeviceID(deviceID, uid)
					if dev != nil && dev.UserID != "" {
						if !common.IsActualDevice(dev.DeviceType) {
							log.Errorf("sendToDevice to device for user %s, but device %s is not actual", uid, deviceID)
							continue
						}
					} else {
						//device not exists
						log.Errorf("sendToDevice device for user %s, but device %s is not exists", uid, deviceID)
						continue
					}*/

					//log.Errorf("add sendToDevice event pos %d sender %s type %s uid %s device %s", pos, device.UserID, eventType, uid, deviceID)
					c.stdEventTimeline.AddSTDEventStream(&dataStream, uid, deviceID)
				}
			}
		}
	}

	return http.StatusOK, nil
}

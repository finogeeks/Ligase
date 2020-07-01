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

package rpc

import (
	"bytes"
	"io"
	"net/http"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/nats-io/go-nats"
)

func GetAliasRoomID(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	req *external.GetDirectoryRoomAliasRequest,
	response *external.GetDirectoryRoomAliasResponse,
) error {
	data, err := RpcRequest(rpcClient, destination, cfg.Rpc.FedAliasTopic, req)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}

func GetRoomState(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	req *external.GetFedRoomStateRequest,
	response *gomatrixserverlib.RespState,
) error {
	data, err := RpcRequest(rpcClient, destination, cfg.Rpc.FedRsQryTopic, req)
	if err == nil {
		json.Unmarshal(data, response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}

type DownloadCB struct {
	id string
	ch chan []byte
}

func (cb *DownloadCB) GetTopic() string {
	return cb.id
}

func (cb *DownloadCB) GetCB() nats.MsgHandler {
	return func(msg *nats.Msg) {
		cb.ch <- msg.Data
	}
}

func (cb *DownloadCB) Clean() {
}

type DownloadReader struct {
	ch     chan []byte
	buf    bytes.Buffer
	cb     *DownloadCB
	rpcCli *common.RpcClient
}

func (r *DownloadReader) Read(p []byte) (n int, err error) {
	idx := 0
	for r.buf.Len() > 0 {
		nn, ee := r.buf.Read(p[idx:])
		idx += nn
		if idx >= len(p) {
			return idx, ee
		}
	}
	data := <-r.ch
	if len(data) == 0 {
		r.rpcCli.Unsubscribe(r.cb)
		return idx, io.EOF
	}
	nn := copy(p[idx:], data)
	idx += nn
	if nn < len(data) {
		if len(data)-nn > 0 {
			_, err = r.buf.Write(data[nn:])
			if err != nil {
				return idx, err
			}
		}
	}
	return idx, nil
}

func Download(
	cfg *config.Dendrite,
	rpcCli *common.RpcClient,
	id, destination, mediaID, width, method, fileType string,
) (io.Reader, int, http.Header, error) {
	req := &external.GetFedDownloadRequest{
		ID:       id,
		FileType: fileType,
		MediaID:  mediaID,
		Width:    width,
		Method:   method,
	}
	ch := make(chan []byte, 64)
	cb := &DownloadCB{id: id, ch: ch}
	rpcCli.SubRaw(cb)
	data, err := RpcRequest(rpcCli, destination, cfg.Rpc.FedDownloadTopic, req)
	if err != nil {
		log.Errorf("federation download rpc error: %v", err)
		rpcCli.Unsubscribe(cb)
		close(ch)
		return nil, 500, nil, err
	}

	reader := &DownloadReader{ch: ch, cb: cb, rpcCli: rpcCli}
	log.Infof("federation download: %v", req)

	resp := external.GetFedDownloadResponse{}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		log.Errorf("federation download response header error %v, data: %v", err, data)
		return nil, 500, nil, err
	}

	return reader, resp.StatusCode, resp.Header, nil
}

func MakeJoin(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	roomID, userID string, ver []string,
	response *gomatrixserverlib.RespMakeJoin,
) error {
	var req external.GetMakeJoinRequest
	req.RoomID = roomID
	req.UserID = userID
	req.Ver = ver
	data, err := RpcRequest(rpcClient, destination, cfg.Rpc.FedMakeJoinTopic, &req)
	if err != nil {
		log.Errorf("federation make join error: %v", err)
		return err
	}
	json.Unmarshal(data, response)
	return nil
}

func SendJoin(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	roomID, eventID string,
	event gomatrixserverlib.Event,
	resp *gomatrixserverlib.RespSendJoin,
) error {
	var req external.PutSendJoinRequest
	req.RoomID = roomID
	req.EventID = eventID
	req.Event = event
	data, err := RpcRequest(rpcClient, destination, cfg.Rpc.FedSendJoinTopic, &req)
	if err != nil {
		log.Errorf("federation send join error: %v", err)
		return err
	}
	json.Unmarshal(data, resp)
	return nil
}

func MakeLeave(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	roomID, userID string,
	response *gomatrixserverlib.RespMakeLeave,
) error {
	var req external.GetMakeLeaveRequest
	req.RoomID = roomID
	req.UserID = userID
	data, err := RpcRequest(rpcClient, destination, cfg.Rpc.FedMakeLeaveTopic, &req)
	if err != nil {
		log.Errorf("federation make leave error: %v", err)
		return err
	}
	json.Unmarshal(data, response)
	return nil
}

func SendLeave(
	cfg *config.Dendrite,
	rpcClient *common.RpcClient,
	destination string,
	roomID, eventID string,
	event gomatrixserverlib.Event,
	resp *gomatrixserverlib.RespSendLeave,
) error {
	var req external.PutSendLeaveRequest
	req.RoomID = roomID
	req.EventID = eventID
	req.Event = event
	data, err := RpcRequest(rpcClient, destination, cfg.Rpc.FedSendLeaveTopic, &req)
	if err != nil {
		log.Errorf("federation send leave error: %v", err)
		return err
	}
	json.Unmarshal(data, resp)
	return nil
}

func SendInvite(
	cfg *config.Dendrite,
	rpcCli *common.RpcClient,
	destination string,
	event gomatrixserverlib.Event,
	response *gomatrixserverlib.RespInvite,
) error {
	data, err := RpcRequest(rpcCli, destination, cfg.Rpc.FedInviteTopic, event)
	if err == nil {
		json.Unmarshal(data, &response)
		return nil
	}

	log.Infof("federation rpc request failed, err: %v", err)
	return err
}

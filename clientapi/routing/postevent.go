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

package routing

import (
	"context"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

type sendEventResponse struct {
	EventID string `json:"event_id"`
}

func PostEvent(
	ctx context.Context,
	req *external.PutRoomStateByTypeWithTxnID,
	userID, deviceID, IP string,
	roomID, eventType string,
	txnID, stateKey *string,
	cfg config.Dendrite,
	rpcCli roomserverapi.RoomserverRPCAPI,
	cache service.Cache,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	start := time.Now().UnixNano()
	last := start
	var r map[string]interface{}
	json.Unmarshal(req.Content, &r)
	if val, ok := r["membership"]; ok {
		if val == "kick" {
			r["membership"] = "leave"
		}
	}
	var txnAndDeviceID *roomservertypes.TransactionID
	if txnID != nil {
		txnAndDeviceID = &roomservertypes.TransactionID{
			TransactionID: *txnID,
			DeviceID:      deviceID,
			IP:            IP,
		}
		key := roomID + eventType + (*txnID)
		eventIDRef, ok := cache.GetTxnID(roomID, key)
		if ok == true {
			return http.StatusOK, &external.PutRoomStateByTypeWithTxnIDResponse{
				EventID: eventIDRef,
			}
		}
	}else{
		txnAndDeviceID = &roomservertypes.TransactionID{
			TransactionID: "",
			DeviceID:      deviceID,
			IP:            IP,
		}
	}
	log.Infof("PostEvent txnid:%s userID %s roomID %s", txnAndDeviceID.TransactionID, userID, roomID)
	builder := gomatrixserverlib.EventBuilder{
		Sender:   userID,
		RoomID:   roomID,
		Type:     eventType,
		StateKey: stateKey,
	}
	err := builder.SetContent(r)
	if err != nil {
		log.Errorf("PostEvent SetContent error, txnid:%s userID %s roomID %s eventType %s stateKey %s err %v", txnAndDeviceID.TransactionID, userID, roomID, eventType, stateKey, err)
		return httputil.LogThenErrorCtx(ctx, err)
	}

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	err = rpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		log.Errorf("PostEvent QueryRoomState error, txnid:%s userID %s roomID %s err %v", txnAndDeviceID.TransactionID, userID, roomID,  err)
		return http.StatusNotFound, jsonerror.NotFound(err.Error()) //err
	}

	domainID, _ := common.DomainFromID(userID)
	e, err := common.BuildEvent(&builder, domainID, cfg, idg)
	log.Infof("------------------------PostEvent txnId:%s build-event %v", txnAndDeviceID.TransactionID, (time.Now().UnixNano()-last)/1000)
	last = time.Now().UnixNano()
	if err != nil {
		log.Errorf("PostEvent BuildEvent error, txnid:%s userID %s roomID %s err %v", txnAndDeviceID.TransactionID, userID, roomID,  err)
		return httputil.LogThenErrorCtx(ctx, err)
	}

	// check to see if this user can perform this operation
	if eventType == "m.room.create" {
		log.Infof("------------------------PostEvent %v", *e)
	}
	if err = gomatrixserverlib.Allowed(*e, &queryRes); err != nil {
		log.Errorf("PostEvent Allowed error, txnid:%s userID %s roomID %s err %v", txnAndDeviceID.TransactionID, userID, roomID,  err)
		return http.StatusForbidden, jsonerror.Forbidden(err.Error()) // TODO: Is this error string comprehensible to the client?
	}

	log.Debugf("------------------------PostEvent check-event-allowed %v", (time.Now().UnixNano()-last)/1000)
	last = time.Now().UnixNano()

	// pass the new event to the roomserver
	rawEvent := roomserverapi.RawEvent{
		RoomID: roomID,
		Kind:   roomserverapi.KindNew,
		TxnID:  txnAndDeviceID,
		Trust:  true,
		BulkEvents: roomserverapi.BulkEvent{
			Events:  []gomatrixserverlib.Event{*e},
			SvrName: domainID,
		},
		Query: []string{"post_event", eventType},
	}
	//write event to kafka
	_, err = rpcCli.InputRoomEvents(ctx, &rawEvent)

	if err != nil {
		log.Errorf("PostEvent error, userID %s roomID %s txnId:%s err %v", userID, roomID, txnAndDeviceID.TransactionID, err)
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if txnID != nil {
		cache.PutTxnID(roomID, roomID+eventType+(*txnID), e.EventID())
	}

	log.Debugf("------------------------PostEvent send-event-to-server %v", (time.Now().UnixNano()-last)/1000)
	log.Infof("------------------------PostEvent all %v remote:%s dev:%s eventId:%s txnId:%s", (time.Now().UnixNano()-start)/1000, IP, deviceID, e.EventID(), txnAndDeviceID.TransactionID)

	return http.StatusOK, &external.PutRoomStateByTypeWithTxnIDResponse{
		EventID: e.EventID(),
	}
}

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
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/model/roomservertypes"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/model/service/roomserverapi"
	"github.com/finogeeks/ligase/plugins/message/external"

	log "github.com/finogeeks/ligase/skunkworks/log"
)

func RedactEvent(
	ctx context.Context, //
	//req *external.PutRedactEventRequest,
	content []byte,
	userID, deviceID string,
	roomID string,
	txnID, stateKey *string,
	cfg config.Dendrite,
	localCache service.LocalCache,
	rsRpcCli roomserverapi.RoomserverRPCAPI,
	redacts string,
	eventType string,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	//eventType := "m.room.redaction"
	start := time.Now().UnixNano()
	last := start
	// var r map[string]interface{} //request body
	// resErr := httputil.UnmarshalJSONRequest(req, &r)
	// if resErr != nil {
	// 	return *resErr
	// }

	var txnAndDeviceID *roomservertypes.TransactionID
	if txnID != nil {
		txnAndDeviceID = &roomservertypes.TransactionID{
			TransactionID: *txnID,
			DeviceID:      deviceID,
		}
		key := roomID + eventType + (*txnID)
		eventIDRef, ok := localCache.Get(0, key)
		if ok == true {
			return 200, &external.PutRedactEventResponse{
				EventID: eventIDRef.(string),
			}
		}
	} else {
		key := roomID + eventType + redacts
		eventIDRef, ok := localCache.Get(0, key)
		if ok == true {
			return 200, &external.PutRedactEventResponse{
				EventID: eventIDRef.(string),
			}
		}
	}

	queryRoomEventByIDRequest := roomserverapi.QueryRoomEventByIDRequest{
		EventID: redacts,
		RoomID:  roomID,
	}
	var queryRoomEventByIDResponse roomserverapi.QueryRoomEventByIDResponse

	if err := rsRpcCli.QueryRoomEventByID(ctx, &queryRoomEventByIDRequest, &queryRoomEventByIDResponse); err != nil {
		return 404, jsonerror.Unknown(err.Error())
	}

	if queryRoomEventByIDResponse.Event == nil {
		return 404, jsonerror.NotFound("can't find original event")
	}

	log.Infof("------------------------RedactEvent get target ev %s sender %v", queryRoomEventByIDResponse.Event, queryRoomEventByIDResponse.Event.Sender())

	builder := gomatrixserverlib.EventBuilder{
		Sender:        userID,
		RoomID:        roomID,
		Type:          eventType,
		StateKey:      stateKey,
		Redacts:       redacts,
		RedactsSender: queryRoomEventByIDResponse.Event.Sender(),
	}
	// err := builder.SetContent(req)
	// if err != nil {
	// 	return httputil.LogThenErrorCtx(ctx, err)
	// }
	builder.Content = content

	var queryRes roomserverapi.QueryRoomStateResponse
	var queryReq roomserverapi.QueryRoomStateRequest
	queryReq.RoomID = roomID
	err := rsRpcCli.QueryRoomState(ctx, &queryReq, &queryRes)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if queryRes.RoomExists == false {
		return http.StatusNotFound, jsonerror.NotFound("Room does not exist")
	}

	domainID, _ := common.DomainFromID(userID)
	e, err := common.BuildEvent(&builder, domainID, cfg, idg)
	log.Debugf("------------------------RedactEvent build-event %v", (time.Now().UnixNano()-last)/1000)

	e.SetRedactEventSender(queryRoomEventByIDResponse.Event.Sender())
	log.Debugf("------------------------RedactEvent set redact sender %v, sender %v", queryRoomEventByIDResponse.Event.Sender(), e.RedactEventSender())
	last = time.Now().UnixNano()

	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if err = gomatrixserverlib.Allowed(*e, &queryRes); err != nil {
		return http.StatusForbidden, jsonerror.Forbidden(err.Error()) // TODO: Is this error string comprehensible to the client?
	}

	log.Debugf("------------------------RedactEvent check-event-allowed %v", (time.Now().UnixNano()-last)/1000)
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
		Query: []string{"redact", ""},
	}
	//write event to kafka
	_, err = rsRpcCli.InputRoomEvents(context.Background(), &rawEvent)
	if err != nil {
		return httputil.LogThenErrorCtx(ctx, err)
	}

	if txnID != nil {
		localCache.Put(0, roomID+eventType+(*txnID), e.EventID())
	} else {
		localCache.Put(0, roomID+eventType+e.EventID(), e.EventID())
	}

	log.Debugf("------------------------RedactEvent send-event-to-server %v", (time.Now().UnixNano()-last)/1000)
	log.Infof("------------------------RedactEvent all %v", (time.Now().UnixNano()-start)/1000)

	return http.StatusOK, &external.PutRedactEventResponse{
		EventID: e.EventID(),
	}
}

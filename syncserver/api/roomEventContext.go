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
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/apiconsumer"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/jsonerror"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/feedstypes"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/model/syncapitypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncserver/extra"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetEventContext{})
}

type ReqGetEventContext struct{}

func (ReqGetEventContext) GetRoute() string       { return "/rooms/{roomID}/context/{eventID}" }
func (ReqGetEventContext) GetMetricsName() string { return "room_event_context" }
func (ReqGetEventContext) GetMsgType() int32      { return internals.MSG_GET_ROOM_EVENT_CONTEXT }
func (ReqGetEventContext) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetEventContext) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetEventContext) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetEventContext) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetEventContext) NewRequest() core.Coder {
	return new(external.GetRoomEventContextRequest)
}
func (ReqGetEventContext) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomEventContextRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventID = vars["eventID"]
	}

	req.ParseForm()
	limitStr := req.URL.Query().Get("limit")
	if limitStr == "" {
		msg.Limit = 50
	} else {
		msg.Limit, _ = strconv.ParseInt(limitStr, 10, 64)
	}
	return nil
}
func (ReqGetEventContext) NewResponse(code int) core.Coder {
	return make(internals.JSONMap)
}

type GetMessagesSource struct {
	c             *InternalMsgConsumer
	rs            *repos.RoomState
	tl            *feedstypes.TimeLines
	createEv      *feedstypes.StreamEvent
	roomMinStream int64
}

func (ReqGetEventContext) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomEventContextRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	userID := device.UserID
	roomID := req.RoomID
	eventID := req.EventID
	limit := req.Limit
	log.Debugf("get context process, room: %s, event: %s, limit: %d", roomID, eventID, limit)

	c.rsTimeline.LoadStreamStates(ctx, roomID, true)
	rs := c.rsCurState.GetRoomState(roomID)
	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	_, isJoin := rs.GetJoinMap().Load(userID)
	_, isLeave := rs.GetLeaveMap().Load(userID)
	if isJoin == false && isLeave == false {
		return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room or just forget the room")
	}

	roomMinStream := c.rmHsTimeline.GetRoomMinStream(ctx, roomID)

	// get fromPos and fromTs by eventID
	baseEvent, offsets, err := c.db.StreamEvents(ctx, []string{eventID})
	if err != nil || len(baseEvent) <= 0 || len(offsets) <= 0 {
		log.Warnf("cannot find event, eventID: %s, ret events: %v, offsets: %v", eventID, baseEvent, offsets)
		return http.StatusNotFound, jsonerror.NotFound("cannot find event")
	}
	fromTs := int64(baseEvent[0].OriginServerTS)
	fromPos := offsets[0]

	visibilityTime := c.settings.GetMessageVisilibityTime()
	nowTs := time.Now().Unix()
	if visibilityTime > 0 {
		ts := int64(baseEvent[0].OriginServerTS) / 1000
		if ts+visibilityTime < nowTs {
			log.Infof("ReqGetEventContext skip event %s", eventID)
			return http.StatusNotFound, jsonerror.NotFound("cannot find event")
		}
	}

	// get feed
	tl := c.rmHsTimeline.GetHistory(ctx, roomID)
	if tl == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room history")
	}

	createEv := rs.GetState(gomatrixserverlib.MRoomCreate, "")

	// split into backward and forward dir
	backwardLimit := int64(math.Floor(float64(limit / 2.0)))
	forwardLimit := limit - backwardLimit

	source := GetMessagesSource{c, rs, tl, createEv, roomMinStream}
	bwEvents, startPos, startTs, err := source.getMessages(ctx, userID, roomID, "b", fromPos-1, fromTs, backwardLimit)
	if err != nil {
		log.Errorln("get context process, get event before error: ", err.Error())
		return http.StatusInternalServerError, jsonerror.Unknown("failed to get event before")
	}

	fwEvents, endPos, endTs, err := source.getMessages(ctx, userID, roomID, "f", fromPos+1, fromTs, forwardLimit)
	if err != nil {
		log.Errorln("get context process, get event after error: ", err.Error())
		return http.StatusInternalServerError, jsonerror.Unknown("failed to get event after")
	}

	// get current state events
	_, stateEvents := source.c.rsTimeline.GetStateEvents(ctx, roomID, endPos)
	// append hint
	extra.ExpandMessages(&baseEvent[0], userID, c.rsCurState, c.displayNameRepo)

	resp := new(syncapitypes.ContextEventResp)
	resp.EvsBefore = bwEvents
	resp.EvsAfter = fwEvents
	resp.Event = baseEvent[0]
	resp.State = stateEvents
	resp.Start = common.BuildPreBatch(startPos, startTs)
	resp.End = common.BuildPreBatch(endPos, endTs)

	return http.StatusOK, resp
}

func (source GetMessagesSource) getMessages(
	ctx context.Context,
	userID, roomID, dir string, fromPos, fromTs, limit int64,
) ([]gomatrixserverlib.ClientEvent, int64, int64, error) {
	var endPos, endTs int64
	var err error

	outputRoomEvents := []gomatrixserverlib.ClientEvent{}
	outputRecords := make(map[string]int)
	feedLower, feedUpper := source.tl.GetFeedRange()

	if fromPos > feedUpper && dir == "f" { // latest pos, no more data
		endPos = fromPos
		endTs = fromTs
	} else if source.createEv != nil && fromPos > 0 && fromPos <= source.createEv.GetOffset() && dir == "b" { // we should get nothing before the room is created
		endPos = fromPos
		endTs = fromTs
	} else if fromPos < feedLower || fromPos <= source.roomMinStream { // use db
		outputRoomEvents, endPos, endTs, err = source.getFromDB(ctx, userID, roomID, dir, fromPos, fromTs, int(limit))
		log.Debugf("get context [%s dir] from cache, but out of range, get from db, endPos: %d", dir, endPos)
	} else { // use cache
		cacheLoaded := true
		visibilityTime := source.c.settings.GetMessageVisilibityTime()
		nowTs := time.Now().Unix()
		if dir == "b" {
			source.tl.RAtomic(func(data *feedstypes.TimeLinesAtomicData) {
				if fromPos < data.Lower || fromPos <= source.roomMinStream {
					cacheLoaded = false
					return
				}
				for i := data.End - 1; i >= data.Start; i-- {
					feed := data.Data[i%data.Size]
					if feed == nil {
						log.Errorf("RoomMsgRpcConsumer.processOnRoomContext get feed nil offset %d", i)
						source.tl.Console()
						continue
					}
					stream := feed.(*feedstypes.StreamEvent)
					ev := *stream.GetEv()

					if stream.GetOffset() <= fromPos && stream.GetOffset() >= source.roomMinStream {
						if ((source.rs.CheckEventVisibility(userID, int64(stream.Ev.OriginServerTS)) && !common.IsExtEvent(&ev)) || common.IsStateClientEv(&ev)) && stream.GetOffset() > source.roomMinStream {
							skipEv := false
							if visibilityTime > 0 {
								ts := int64(ev.OriginServerTS) / 1000
								if ts+visibilityTime < nowTs && !common.IsStateClientEv(&ev) {
									log.Debugf("GetMessagesSource.getMessages cache skip event %s, ts: %d", ev.EventID, ev.OriginServerTS)
									skipEv = true
								}
							}
							if !skipEv {
								extra.ExpandMessages(&ev, userID, source.c.rsCurState, source.c.displayNameRepo)

								// dereplication
								if idx, ok := outputRecords[ev.EventID]; ok {
									log.Warnf("get context forward from cache, found replicate events, eventID: %d, index: %d", ev.EventID, idx)
									outputRoomEvents[idx] = ev
								} else {
									outputRoomEvents = append(outputRoomEvents, ev)
									outputRecords[ev.EventID] = len(outputRoomEvents) - 1
									limit = limit - 1
								}
							}
						}
						if stream.GetOffset() == source.roomMinStream {
							endPos = stream.GetOffset()
						} else {
							endPos = stream.GetOffset() - 1
							endTs = int64(stream.Ev.OriginServerTS)
						}

						// endPos = stream.GetOffset() - 1
						log.Infof("get context backward from cache, stream.Offset: %d endPos: %d", stream.GetOffset(), endPos)
						// endTs = int64(stream.Ev.OriginServerTS)

						if limit == 0 {
							break
						}
					}
				}
				log.Debugf("get context backward from cache, endPos: %d, data.Lower: %d, minStream: %d", endPos, data.Lower, source.roomMinStream)

				if endPos == 0 {
					endPos = data.Lower - 1
					if endPos < source.roomMinStream {
						endPos = source.roomMinStream
					}
					endTs = fromTs
					log.Debugf("get context backward from cache, but endPos is 0, now is : %d", endPos)
				}
				log.Debugf("get context backward from cache, endPos: %d", endPos)
			})
		} else {
			source.tl.RAtomic(func(data *feedstypes.TimeLinesAtomicData) {
				if fromPos < data.Lower || fromPos <= source.roomMinStream {
					cacheLoaded = false
					return
				}
				for i := data.Start; i < data.End; i++ {
					feed := data.Data[i%data.Size]
					if feed == nil {
						log.Errorf("RoomMsgRpcConsumer.processOnRoomContext get feed nil offset %d", i)
						source.tl.Console()
						continue
					}
					stream := feed.(*feedstypes.StreamEvent)
					ev := *stream.GetEv()

					if stream.GetOffset() >= fromPos && stream.GetOffset() >= source.roomMinStream {
						skipEv := false
						if (source.rs.CheckEventVisibility(userID, int64(stream.Ev.OriginServerTS)) && !common.IsExtEvent(&ev)) || common.IsStateClientEv(&ev) {
							if visibilityTime > 0 {
								ts := int64(ev.OriginServerTS) / 1000
								if ts+visibilityTime < nowTs && !common.IsStateClientEv(&ev) {
									skipEv = true
								}
							}
							if !skipEv {
								extra.ExpandMessages(&ev, userID, source.c.rsCurState, source.c.displayNameRepo)

								// dereplication
								if idx, ok := outputRecords[ev.EventID]; ok {
									log.Warnf("get context forward from cache, found replicate events, eventID: %d, index: %d", ev.EventID, idx)
									outputRoomEvents[idx] = ev
								} else {
									outputRoomEvents = append(outputRoomEvents, ev)
									outputRecords[ev.EventID] = len(outputRoomEvents) - 1
								}
							}
						}
						if !skipEv {
							limit = limit - 1
						}
						endPos = stream.GetOffset() + 1
						log.Debugf("get context forward from cache, stream.Offset: %d", stream.GetOffset())
						endTs = int64(stream.Ev.OriginServerTS)

						if limit == 0 {
							break
						}
					}
				}
				log.Debugf("get context forward from cache, endPos: %d, data.Lower: %d, minStream: %d", endPos, data.Lower, source.roomMinStream)

				if endPos == 0 {
					endPos = data.Upper + 1
					endTs = fromTs
					log.Debugf("get context forward from cache, but endPos is 0, now is : %d", endPos)
				}
				log.Debugf("get context forward from cache, endPos: %d", endPos)
			})
		}
		if !cacheLoaded {
			outputRoomEvents, endPos, endTs, err = source.getFromDB(ctx, userID, roomID, dir, fromPos, fromTs, int(limit))
			log.Debugf("get context [%s dir], load cache failed , get from db, endPos: %d", dir, endPos)
		}
	}

	return outputRoomEvents, endPos, endTs, err
}

func (source GetMessagesSource) getFromDB(
	ctx context.Context,
	userID, roomID, dir string,
	fromPos, fromTs int64, limit int,
) ([]gomatrixserverlib.ClientEvent, int64, int64, error) {
	source.c.rsTimeline.QueryHitCounter.WithLabelValues("db", "RoomEventContext", "getEvents").Inc()

	outputRoomEvents := []gomatrixserverlib.ClientEvent{}

	events, offsets, _, err, endPos, endTs := source.c.db.SelectEventsByDir(ctx, userID, roomID, dir, fromPos, limit)
	if err != nil {
		return outputRoomEvents, fromPos, fromTs, err
	}
	if len(events) == 0 {
		return outputRoomEvents, fromPos, fromTs, nil
	}

	visibilityTime := source.c.settings.GetMessageVisilibityTime()
	nowTs := time.Now().Unix()
	for i := range events {
		if (dir == "b" && ((fromPos >= 0 && offsets[i] <= fromPos) || (fromPos < 0 && (offsets[i] > 0 || offsets[i] <= fromPos)))) || (dir == "f" && offsets[i] >= fromPos) {
			if (source.rs.CheckEventVisibility(userID, int64(events[i].OriginServerTS)) && !common.IsExtEvent(&events[i])) || common.IsStateClientEv(&events[i]) {
				if visibilityTime > 0 {
					ts := int64(events[i].OriginServerTS) / 1000
					if ts+visibilityTime < nowTs && !common.IsStateClientEv(&events[i]) {
						log.Debugf("GetMessagesSource select from db skip event %s, ts: %d", events[i].EventID, events[i].OriginServerTS)
						continue
					}
				}
				extra.ExpandMessages(&events[i], userID, source.c.rsCurState, source.c.displayNameRepo)
				outputRoomEvents = append(outputRoomEvents, events[i])
			}
		}
	}

	if dir == "b" {
		if len(outputRoomEvents) == 0 && endPos != 0 && source.createEv != nil {
			endPos = source.createEv.GetOffset()
		} else {
			endPos = endPos - 1
		}
	} else {
		endPos = endPos + 1
	}

	return outputRoomEvents, endPos, endTs, nil
}

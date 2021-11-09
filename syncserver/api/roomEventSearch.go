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
	"sort"
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
	apiconsumer.SetServices("sync_server_api")
	apiconsumer.SetAPIProcessor(ReqGetEventSearch{})
}

type ReqGetEventSearch struct{}

func (ReqGetEventSearch) GetRoute() string       { return "/rooms/{roomID}/search/{eventID}" }
func (ReqGetEventSearch) GetMetricsName() string { return "room_event_context" }
func (ReqGetEventSearch) GetMsgType() int32      { return internals.MSG_GET_ROOM_EVENT_SEARCH }
func (ReqGetEventSearch) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetEventSearch) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetEventSearch) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetEventSearch) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetEventSearch) NewRequest() core.Coder {
	return new(external.GetRoomEventSearchRequest)
}
func (ReqGetEventSearch) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomEventSearchRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
		msg.EventID = vars["eventID"]
	}

	req.ParseForm()
	size := req.URL.Query().Get("size")
	if size == "" {
		msg.Size = 15
	} else {
		msg.Size, _ = strconv.ParseInt(size, 10, 64)
	}
	return nil
}
func (ReqGetEventSearch) NewResponse(code int) core.Coder {
	return make(internals.JSONMap)
}
func (ReqGetEventSearch) CalcInstance(msg core.Coder, device *authtypes.Device, cfg *config.Dendrite) []uint32 {
	req := msg.(*external.GetRoomEventSearchRequest)
	return []uint32{common.CalcStringHashCode(req.RoomID) % cfg.MultiInstance.SyncServerTotal}
}

type SearchMessagesSource struct {
	c             *InternalMsgConsumer
	rs            *repos.RoomState
	tl            *feedstypes.TimeLines
	createEv      *feedstypes.StreamEvent
	roomMinStream int64
}

func (ReqGetEventSearch) Process(consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomEventSearchRequest)

	userID := device.UserID
	roomID := req.RoomID
	eventID := req.EventID
	size := req.Size
	log.Debugf("search event process, room: %s, event: %s, size: %d", roomID, eventID, size)

	c.rsTimeline.LoadStreamStates(roomID, true)
	rs := c.rsCurState.GetRoomState(roomID)
	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	_, isJoin := rs.GetJoinMap().Load(userID)
	_, isLeave := rs.GetLeaveMap().Load(userID)
	if isJoin == false && isLeave == false {
		return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room or just forget the room")
	}

	roomMinStream := c.rmHsTimeline.GetRoomMinStream(roomID)

	ctx := context.TODO()

	// get fromPos and fromTs by eventID
	baseEvent, offsets, err := c.db.StreamEvents(ctx, []string{eventID})
	if err != nil || len(baseEvent) <= 0 || len(offsets) <= 0 {
		log.Warnf("cannot find event, eventID: %s, ret events: %v, offsets: %v", eventID, baseEvent, offsets)
		return http.StatusNotFound, jsonerror.NotFound("cannot find event")
	}
	fromTs := int64(baseEvent[0].OriginServerTS)
	fromPos := offsets[0]

	visibilityTime := c.GetCfg().Sync.Visibility
	nowTs := time.Now().Unix()
	if visibilityTime > 0 {
		ts := int64(baseEvent[0].OriginServerTS) / 1000
		if ts+visibilityTime < nowTs {
			log.Infof("ReqGetEventSearch skip event %s", eventID)
			return http.StatusNotFound, jsonerror.NotFound("cannot find event")
		}
	}

	// get feed
	tl := c.rmHsTimeline.GetHistory(roomID)
	if tl == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room history")
	}

	createEv := rs.GetState(gomatrixserverlib.MRoomCreate, "")

	countBefore, err := c.db.SelectEventCountBefore(ctx, roomID, offsets[0])
	if err != nil {
		log.Warnf("cannot count event, roomID: %s, offsets: %v, err: %s", roomID, offsets[0], err)
		return http.StatusInternalServerError, jsonerror.Unknown("count event before error")
	}

	count, err := c.db.SelectEventCount(ctx, roomID)
	if err != nil {
		log.Warnf("cannot count event, roomID: %s", roomID)
		return http.StatusInternalServerError, jsonerror.Unknown("count event error")
	}

	//total := int64(int64(math.Ceil(float64(count) / float64(req.Size))))
	page := ((countBefore) / req.Size) + 1
	idxPage := (countBefore) % req.Size

	// split into backward and forward dir
	backwardLimit := idxPage
	forwardLimit := req.Size - backwardLimit - 1

	source := SearchMessagesSource{c, rs, tl, createEv, roomMinStream}
	bwEvents, _, err := source.getMessages(userID, roomID, "b", fromPos-1, fromTs, backwardLimit)
	if err != nil {
		log.Errorln("search event process, get event before error: ", err.Error())
		return http.StatusInternalServerError, jsonerror.Unknown("failed to get event before")
	}

	fwEvents, endPos, err := source.getMessages(userID, roomID, "f", fromPos+1, fromTs, forwardLimit)
	if err != nil {
		log.Errorln("search event process, get event after error: ", err.Error())
		return http.StatusInternalServerError, jsonerror.Unknown("failed to get event after")
	}

	// get current state events
	_, stateEvents := source.c.rsTimeline.GetStateEvents(roomID, endPos)
	// append hint
	extra.ExpandMessages(&baseEvent[0], userID, c.rsCurState, c.displayNameRepo)

	resp := new(syncapitypes.SearchEventResp)
	resp.Total = int(count)
	resp.Page = int(page)
	resp.Chunk = make([]gomatrixserverlib.ClientEvent, 0, len(bwEvents)+len(baseEvent)+len(fwEvents))
	resp.Chunk = append(resp.Chunk, bwEvents...)
	resp.Chunk = append(resp.Chunk, baseEvent...)
	resp.Chunk = append(resp.Chunk, fwEvents...)
	resp.Size = len(resp.Chunk)
	resp.State = stateEvents

	sort.Slice(resp.Chunk, func(i, j int) bool {
		return resp.Chunk[i].EventOffset < resp.Chunk[j].EventOffset
	})

	return http.StatusOK, resp
}

func (source SearchMessagesSource) getMessages(
	userID, roomID, dir string, fromPos, fromTs, limit int64,
) ([]gomatrixserverlib.ClientEvent, int64, error) {
	if limit <= 0 {
		return nil, 0, nil
	}
	var endPos int64
	var err error

	outputRoomEvents := []gomatrixserverlib.ClientEvent{}
	outputRecords := make(map[string]int)
	feedLower, feedUpper := source.tl.GetFeedRange()

	if fromPos > feedUpper && dir == "f" { // latest pos, no more data
		endPos = fromPos
	} else if source.createEv != nil && fromPos > 0 && fromPos <= source.createEv.GetOffset() && dir == "b" { // we should get nothing before the room is created
		endPos = fromPos
	} else if fromPos < feedLower || fromPos <= source.roomMinStream { // use db
		outputRoomEvents, endPos, err = source.getFromDB(context.TODO(), userID, roomID, dir, fromPos, fromTs, int(limit))
		log.Debugf("search event [%s dir] from cache, but out of range, get from db, endPos: %d", dir, endPos)
	} else { // use cache
		cacheLoaded := true
		visibilityTime := source.c.GetCfg().Sync.Visibility
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
									log.Debugf("SearchMessagesSource.getMessages cache skip event %s, ts: %d", ev.EventID, ev.OriginServerTS)
									skipEv = true
								}
							}
							if !skipEv {
								extra.ExpandMessages(&ev, userID, source.c.rsCurState, source.c.displayNameRepo)

								// dereplication
								if idx, ok := outputRecords[ev.EventID]; ok {
									log.Warnf("search event forward from cache, found replicate events, eventID: %d, index: %d", ev.EventID, idx)
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
						}

						// endPos = stream.GetOffset() - 1
						log.Infof("search event backward from cache, stream.Offset: %d endPos: %d", stream.GetOffset(), endPos)
						// endTs = int64(stream.Ev.OriginServerTS)

						if limit == 0 {
							break
						}
					}
				}
				log.Debugf("search event backward from cache, endPos: %d, data.Lower: %d, minStream: %d", endPos, data.Lower, source.roomMinStream)

				if endPos == 0 {
					endPos = data.Lower - 1
					if endPos < source.roomMinStream {
						endPos = source.roomMinStream
					}
					log.Debugf("search event backward from cache, but endPos is 0, now is : %d", endPos)
				}
				log.Debugf("search event backward from cache, endPos: %d", endPos)
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
									log.Warnf("search event forward from cache, found replicate events, eventID: %d, index: %d", ev.EventID, idx)
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
						log.Debugf("search event forward from cache, stream.Offset: %d", stream.GetOffset())

						if limit == 0 {
							break
						}
					}
				}
				log.Debugf("search event forward from cache, endPos: %d, data.Lower: %d, minStream: %d", endPos, data.Lower, source.roomMinStream)

				if endPos == 0 {
					endPos = data.Upper + 1
					log.Debugf("search event forward from cache, but endPos is 0, now is : %d", endPos)
				}
				log.Debugf("search event forward from cache, endPos: %d", endPos)
			})
		}
		if !cacheLoaded {
			outputRoomEvents, endPos, err = source.getFromDB(context.TODO(), userID, roomID, dir, fromPos, fromTs, int(limit))
			log.Debugf("search event [%s dir], load cache failed , get from db, endPos: %d", dir, endPos)
		}
	}

	return outputRoomEvents, endPos, err
}

func (source SearchMessagesSource) getFromDB(
	ctx context.Context,
	userID, roomID, dir string,
	fromPos, fromTs int64, limit int,
) ([]gomatrixserverlib.ClientEvent, int64, error) {
	source.c.rsTimeline.QueryHitCounter.WithLabelValues("db", "RoomEventContext", "getEvents").Inc()

	outputRoomEvents := []gomatrixserverlib.ClientEvent{}

	events, offsets, _, err, endPos, _ := source.c.db.SelectEventsByDir(ctx, userID, roomID, dir, fromPos, limit)
	if err != nil {
		return outputRoomEvents, fromPos, err
	}
	if len(events) == 0 {
		return outputRoomEvents, fromPos, nil
	}

	visibilityTime := source.c.GetCfg().Sync.Visibility
	nowTs := time.Now().Unix()
	for i := range events {
		if (dir == "b" && ((fromPos >= 0 && offsets[i] <= fromPos) || (fromPos < 0 && (offsets[i] > 0 || offsets[i] <= fromPos)))) || (dir == "f" && offsets[i] >= fromPos) {
			if (source.rs.CheckEventVisibility(userID, int64(events[i].OriginServerTS)) && !common.IsExtEvent(&events[i])) || common.IsStateClientEv(&events[i]) {
				if visibilityTime > 0 {
					ts := int64(events[i].OriginServerTS) / 1000
					if ts+visibilityTime < nowTs && !common.IsStateClientEv(&events[i]) {
						log.Debugf("SearchMessagesSource select from db skip event %s, ts: %d", events[i].EventID, events[i].OriginServerTS)
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

	return outputRoomEvents, endPos, nil
}

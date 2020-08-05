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
	"strings"
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
	"github.com/finogeeks/ligase/skunkworks/gomatrix"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/syncserver/extra"
)

func init() {
	apiconsumer.SetAPIProcessor(ReqGetRoomMessages{})
}

type ReqGetRoomMessages struct{}

func (ReqGetRoomMessages) GetRoute() string       { return "/rooms/{roomID}/messages" }
func (ReqGetRoomMessages) GetMetricsName() string { return "room_messages" }
func (ReqGetRoomMessages) GetMsgType() int32      { return internals.MSG_GET_ROOM_MESSAGES }
func (ReqGetRoomMessages) GetAPIType() int8       { return apiconsumer.APITypeAuth }
func (ReqGetRoomMessages) GetMethod() []string {
	return []string{http.MethodGet, http.MethodOptions}
}
func (ReqGetRoomMessages) GetTopic(cfg *config.Dendrite) string { return getProxyRpcTopic(cfg) }
func (ReqGetRoomMessages) GetPrefix() []string                  { return []string{"r0"} }
func (ReqGetRoomMessages) NewRequest() core.Coder {
	return new(external.GetRoomMessagesRequest)
}
func (ReqGetRoomMessages) FillRequest(coder core.Coder, req *http.Request, vars map[string]string) error {
	msg := coder.(*external.GetRoomMessagesRequest)
	if vars != nil {
		msg.RoomID = vars["roomID"]
	}
	req.ParseForm()
	values := req.URL.Query()
	msg.From = values.Get("from")
	msg.To = values.Get("to")
	msg.Dir = values.Get("dir")
	msg.Filter = values.Get("filter")
	msg.Limit = values.Get("limit")
	return nil
}
func (ReqGetRoomMessages) NewResponse(code int) core.Coder {
	return new(syncapitypes.MessageEventResp)
}
func (r ReqGetRoomMessages) Process(ctx context.Context, consumer interface{}, msg core.Coder, device *authtypes.Device) (int, core.Coder) {
	c := consumer.(*InternalMsgConsumer)
	req := msg.(*external.GetRoomMessagesRequest)
	if !common.IsRelatedRequest(req.RoomID, c.Cfg.MultiInstance.Instance, c.Cfg.MultiInstance.Total, c.Cfg.MultiInstance.MultiWrite) {
		return internals.HTTP_RESP_DISCARD, jsonerror.MsgDiscard("msg discard")
	}

	userID := device.UserID
	roomID := req.RoomID

	states := c.rsTimeline.GetStateStreams(ctx, roomID)
	if states == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	userId := userID
	rs := c.rsCurState.GetRoomState(roomID)

	if rs == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room state")
	}

	_, isJoin := rs.GetJoinMap().Load(userId)
	_, isLeave := rs.GetLeaveMap().Load(userId)

	if isJoin == false && isLeave == false {
		return http.StatusForbidden, jsonerror.Forbidden("You aren't a member of the room and weren't previously a member of the room or just forget the room")
	}

	var fromPos int64
	var fromTs int64
	var toPos int64
	var toTs int64
	fromStr := req.From
	toStr := req.To
	roomMinStream := c.rmHsTimeline.GetRoomMinStream(ctx, roomID)
	//兼容android
	lastEv := c.rmHsTimeline.GetLastEvent(ctx, roomID)
	if lastEv != nil {
		fromPos = lastEv.Offset
		fromTs = int64(lastEv.Ev.OriginServerTS)
	} else {
		fromPos = math.MaxInt64
		fromTs = math.MaxInt64
	}

	if fromStr != "" && fromStr != "-1" {
		r.parsePosToken(fromStr, &fromPos, &fromTs)
	}
	if toStr != "" {
		r.parsePosToken(toStr, &toPos, &toTs)
	}

	limit, _ := strconv.Atoi(req.Limit)
	if toPos == 0 && limit < 50 {
		limit = 50
	}

	dir := req.Dir

	filterStr := req.Filter
	var evFilter gomatrix.FilterPart
	if filterStr != "" {
		json.Unmarshal([]byte(filterStr), &evFilter)
		if strings.Contains(filterStr, "contains_url") == false {
			evFilter.ContainsURL = true
		}
	} else {
		evFilter.ContainsURL = true
	}

	if evFilter.Limit != nil {
		if *evFilter.Limit > 0 && *evFilter.Limit < limit {
			limit = *evFilter.Limit
		}
	}

	tl := c.rmHsTimeline.GetHistory(ctx, roomID)
	if tl == nil {
		return http.StatusNotFound, jsonerror.NotFound("cannot find room history")
	}

	var endPos int64
	var endTs int64
	start, end := tl.GetRange()
	feedlow, feedup := tl.GetFeedRange()
	outputRoomEvents := []gomatrixserverlib.ClientEvent{}
	outputRecords := make(map[string]int)

	createEv := rs.GetState(gomatrixserverlib.MRoomCreate, "")

	log.Infof("OnRoomMessagesRequest start:%d end:%d low:%d up:%d user:%s limit:%d filter: %s minOffset: %d, dir:%s from:%s to:%s", start, end, feedlow, feedup, userId, limit, filterStr, roomMinStream, dir, fromStr, toStr)

	if toPos > 0 && limit > 0 {
		limit = 0
	}

	if fromPos > feedup && dir == "f" { // latest pos, no more data
		endPos = fromPos
		endTs = fromTs
	} else if createEv != nil && fromPos > 0 && fromPos <= createEv.GetOffset() && dir == "b" { // we should get nothing before the room is created
		endPos = fromPos
		endTs = fromTs
	} else if fromPos < feedlow || fromPos <= roomMinStream { // use db
		log.Infof("get messages from db, fromPos: %d", fromPos)
		_outputRoomEvents, _endPos, _endTs, err := r.selectFromDB(ctx, c, rs, userID, roomID, dir, fromPos, fromTs, toPos, toTs, limit)

		if err != nil {
			return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
		}
		if len(_outputRoomEvents) > 0 {
			outputRoomEvents = _outputRoomEvents
		}
		endTs = _endTs
		if dir == "b" {
			// if len(_outputRoomEvents) == 0 && endPos != 0 && createEv != nil {
			// 	endPos = createEv.GetOffset()
			// } else {
			endPos = _endPos - 1
			// }
		} else {
			endPos = _endPos + 1
		}
	} else { // use cache
		var (
			loadFromDB     = false
			dbFromPos      = fromPos
			dbFromTs       = fromTs
			visibilityTime = c.settings.GetMessageVisilibityTime()
			nowTs          = time.Now().Unix()

			foundAll    = false // loaded all request event from cache
			feeds       []feedstypes.Feed
			low, high   int64
			checkOffset func(streamOffset, border, roomMinStream int64) bool
		)
		if dir == "f" {
			feeds, _, _, low, high = tl.GetAllFeeds()
			checkOffset = func(streamOffset, border, roomMinStream int64) bool {
				return streamOffset >= border && streamOffset >= roomMinStream
			}
		} else {
			feeds, _, _, low, high = tl.GetAllFeedsReverse()
			checkOffset = func(streamOffset, border, roomMinStream int64) bool {
				return streamOffset <= border && streamOffset >= roomMinStream
			}
		}
		log.Infof("get messages %s dir: %s from cache, fromPos: %d", roomID, dir, fromPos)
		for _, feed := range feeds {
			stream := feed.(*feedstypes.StreamEvent)
			ev := *stream.GetEv()

			if !checkOffset(stream.GetOffset(), fromPos, roomMinStream) {
				continue
			}
			if toPos != 0 {
				if dir == "b" && stream.GetOffset() < toPos {
					foundAll = true
					break
				} else if dir == "f" && stream.GetOffset() > toPos {
					foundAll = true
					break
				}
			}
			if (rs.CheckEventVisibility(userId, int64(stream.Ev.OriginServerTS)) || common.IsStateClientEv(&ev)) && stream.GetOffset() > roomMinStream {
				skipEv := false
				if visibilityTime > 0 {
					ts := int64(ev.OriginServerTS) / 1000
					if ts+visibilityTime < nowTs && !common.IsStateClientEv(&ev) {
						log.Debugf("getMessage select from db skip event %s, ts: %d", ev.EventID, ev.OriginServerTS)
						skipEv = true
					}
				}

				if !skipEv {
					extra.ExpandMessages(&ev, userID, c.rsCurState, c.displayNameRepo)

					if idx, ok := outputRecords[ev.EventID]; ok {
						log.Warnf("get messages dir: %s from cache, found replicate events, eventID: %s, eventNID: %d, index: %d", dir, ev.EventID, ev.EventNID, idx)
						outputRoomEvents[idx] = ev
					} else {
						outputRoomEvents = append(outputRoomEvents, ev)
						outputRecords[ev.EventID] = len(outputRoomEvents) - 1
						if toPos == 0 {
							limit = limit - 1
						}
					}
				}
			}
			if dir == "b" {
				if stream.GetOffset() == roomMinStream {
					endPos = stream.GetOffset()
					if endTs == 0 {
						endTs = fromTs
					}
				} else {
					endPos = stream.GetOffset() - 1
					endTs = int64(stream.Ev.OriginServerTS)
				}
			} else {
				endPos = stream.GetOffset() + 1
				endTs = int64(stream.Ev.OriginServerTS)
			}

			if toPos == 0 && limit == 0 {
				foundAll = true
				break
			}
		}
		if limit > 0 || !foundAll {
			loadFromDB = true
			dbFromPos = endPos
			dbFromTs = endTs
			if dbFromPos == 0 {
				dbFromPos = fromPos
				dbFromTs = fromTs
			}
		}

		if loadFromDB {
			_outputRoomEvents, _endPos, _endTs, err := r.selectFromDB(context.TODO(), c, rs, userID, roomID, dir, dbFromPos, dbFromTs, toPos, toTs, limit)
			log.Infof("get messages %s from cache limit, get from db, fromPos: %d, fromTs: %d, toPos: %d, toTs: %d, curLen: %d, limit: %d dbLen: %d", roomID, dbFromPos, dbFromTs, toPos, toTs, len(outputRoomEvents), limit, len(_outputRoomEvents))
			if err != nil {
				return http.StatusInternalServerError, jsonerror.Unknown(err.Error())
			}
			if len(_outputRoomEvents) > 0 {
				outputRoomEvents = append(outputRoomEvents, _outputRoomEvents...)
			}
			endTs = _endTs
			if dir == "b" {
				// if len(_outputRoomEvents) == 0 && endPos != 0 && createEv != nil {
				// 	endPos = createEv.GetOffset()
				// } else {
				endPos = _endPos - 1
				// }
			} else {
				endPos = _endPos + 1
			}
		}
		if endPos == 0 || endPos == 1 || endPos == -1 {
			if dir == "b" {
				endPos = low - 1
				if endPos < roomMinStream {
					endPos = roomMinStream
				}
				endTs = fromTs
			} else {
				endPos = high + 1
				endTs = fromTs
			}
		}
	}

	resp := new(syncapitypes.MessageEventResp)
	resp.Start = common.BuildPreBatch(fromPos, fromTs)
	resp.End = common.BuildPreBatch(endPos, endTs)

	if c.Cfg.UseMessageFilter {
		resp.Chunk = *common.FilterEventTypes(&outputRoomEvents, &evFilter.Types, &evFilter.NotTypes)
	} else {
		resp.Chunk = outputRoomEvents
	}

	return http.StatusOK, resp
}

func (r ReqGetRoomMessages) selectFromDB(
	ctx context.Context,
	c *InternalMsgConsumer,
	rs *repos.RoomState,
	userId, roomID, dir string,
	fromPos int64, fromTs int64,
	toPos, toTs int64, limit int,
) (outputRoomEvents []gomatrixserverlib.ClientEvent, endPos, endTs int64, err error) {
	c.rsTimeline.QueryHitCounter.WithLabelValues("db", "RoomMessages", "getEvents").Inc()

	var (
		events  []gomatrixserverlib.ClientEvent
		offsets []int64
	)

	if limit > 0 {
		events, offsets, _, err, endPos, endTs = c.db.SelectEventsByDir(ctx, userId, roomID, dir, fromPos, limit*2)
	} else {
		events, offsets, _, err, endPos, endTs = c.db.SelectEventsByDirRange(ctx, userId, roomID, dir, fromPos, toPos)
	}
	if err != nil {
		return
	}

	visibilityTime := c.settings.GetMessageVisilibityTime()
	nowTs := time.Now().Unix()

	for idx := range events {
		if (dir == "b" && ((fromPos >= 0 && offsets[idx] <= fromPos) || (fromPos < 0 && (offsets[idx] > 0 || offsets[idx] <= fromPos)))) || (dir == "f" && offsets[idx] >= fromPos) {
			if rs.CheckEventVisibility(userId, int64(events[idx].OriginServerTS)) || common.IsStateClientEv(&events[idx]) == true {
				if visibilityTime > 0 {
					ts := int64(events[idx].OriginServerTS) / 1000
					if ts+visibilityTime < nowTs && !common.IsStateClientEv(&events[idx]) {
						log.Infof("getMessage select from db skip event %s, ts: %d", events[idx].EventID, events[idx].OriginServerTS)
						continue
					}
				}

				extra.ExpandMessages(&events[idx], userId, c.rsCurState, c.displayNameRepo)
				outputRoomEvents = append(outputRoomEvents, events[idx])
				endPos = events[idx].EventOffset
				endTs = int64(events[idx].OriginServerTS)
				if limit > 0 {
					limit--
				}
				if toPos == 0 && limit <= 0 {
					break
				} else if toPos > 0 {
					if dir == "b" && events[idx].EventOffset < toPos {
						break
					} else if dir == "f" && events[idx].EventOffset > toPos {
						break
					}
				}
			} else {
				log.Infof("getMessage select from db check visibility false %s fromPos:%d offset:%d ts:%d", roomID, fromPos, offsets[idx], events[idx].OriginServerTS)
			}
		} else {
			log.Infof("getMessage select from db check offset false %s fromPos:%d offset:%d ts:%d", roomID, fromPos, offsets[idx], events[idx].OriginServerTS)
		}
	}
	return
}

func (r ReqGetRoomMessages) parsePosToken(s string, pos, ts *int64) {
	offsets := strings.Split(s, "_")
	if len(offsets) > 1 {
		for _, val := range offsets {
			items := strings.Split(val, ":")
			switch items[0] {
			case "p":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					*pos = int64(i)
				}
			case "t":
				i, err := strconv.Atoi(items[1])
				if err == nil {
					*ts = int64(i)
				}
			}
		}
	}
	return
}

package exporter

import (
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/plugins/message/external"
	"sync"
)

const (
	ROOM_SCALE_LARGE = "超大"
	ROOM_SCALE_BIG = "大"
	ROOM_SCALE_MIDDLE = "中"
	ROOM_SCALE_SMALL = "小"
)

type SyncServerExporter struct {
	roomCurState *repos.RoomCurStateRepo
	base *basecomponent.BaseDendrite
}

var onceSyncServer sync.Once
var syncServerExporter *SyncServerExporter

func SyncServerExporterMetrics(
	roomCurState *repos.RoomCurStateRepo,
	base *basecomponent.BaseDendrite,
) *SyncServerExporter {
	onceSyncServer.Do(func() {
		syncServerExporter = &SyncServerExporter{
			roomCurState: roomCurState,
			base: base,
		}
	})
	return syncServerExporter
}

func (s *SyncServerExporter) getMetrics() *external.SyncServerMetrics {
	return &external.SyncServerMetrics{
		Instance: s.getInstance(),
		RoomScale: s.getRoomScale(),
	}
}

func (s *SyncServerExporter) getInstance() int {
	return s.base.Cfg.Matrix.InstanceId
}

func (s *SyncServerExporter) getRoomScale() external.RoomScaleMetrics {
	roomScale := external.RoomScaleMetrics{
		Large: external.RoomScale{
			Label: ROOM_SCALE_LARGE,
			Count: 0,
		},
		Big: external.RoomScale{
			Label: ROOM_SCALE_BIG,
			Count: 0,
		},
		Middle: external.RoomScale{
			Label: ROOM_SCALE_MIDDLE,
			Count: 0,
		},
		Small: external.RoomScale{
			Label: ROOM_SCALE_SMALL,
			Count: 0,
		},
	}
	repo := s.roomCurState.GetRoomStateRepo()
	repo.Range(func(k, v interface{}) bool {
		rs := v.(*repos.RoomState)
		count := rs.GetJoinCount()
		if count < s.base.Cfg.Metrics.SyncServer.RoomScale.Small {
			roomScale.Small.Count++
		} else if count >= s.base.Cfg.Metrics.SyncServer.RoomScale.Small && count < s.base.Cfg.Metrics.SyncServer.RoomScale.Middle {
			roomScale.Middle.Count++
		} else if count >= s.base.Cfg.Metrics.SyncServer.RoomScale.Middle && count < s.base.Cfg.Metrics.SyncServer.RoomScale.Large {
			roomScale.Big.Count++
		} else{
			roomScale.Large.Count++
		}
		return true
	})
	return roomScale
}


func GetSyncServerMetrics() *external.SyncServerMetrics {
	return syncServerExporter.getMetrics()
}


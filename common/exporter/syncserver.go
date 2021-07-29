package exporter

import (
	"github.com/finogeeks/ligase/common/basecomponent"
	"github.com/finogeeks/ligase/model/repos"
	"github.com/finogeeks/ligase/plugins/message/external"
	"sync"
)



type SyncServerExporter struct {
	roomCurState *repos.RoomCurStateRepo
	base *basecomponent.BaseDendrite
	data *SyncServerStatic
}

var onceSyncServer sync.Once
var syncServerExporter *SyncServerExporter

type SyncServerStatic struct {
	Msg struct {
		Large 	int64 `json:"large"`
		Big 	int64 `json:"big"`
		Middle  int64 `json:"middle"`
		Small   int64 `json:"small"`
	} `json:"msg"`
}

func SyncServerExporterMetrics(
	roomCurState *repos.RoomCurStateRepo,
	base *basecomponent.BaseDendrite,
) *SyncServerExporter {
	onceSyncServer.Do(func() {
		syncServerExporter = &SyncServerExporter{
			roomCurState: roomCurState,
			base: base,
			data: &SyncServerStatic{},
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
	roomScale := s.roomCurState.GetRoomScale()
	roomScale.Large.MsgCount = s.data.Msg.Large
	roomScale.Big.MsgCount = s.data.Msg.Big
	roomScale.Middle.MsgCount = s.data.Msg.Middle
	roomScale.Small.MsgCount = s.data.Msg.Small
	return roomScale
}

func (s *SyncServerExporter) MsgInc(roomId string){
	rs := s.roomCurState.GetRoomState(roomId)
	if rs == nil {
		return
	}
	count := rs.GetJoinCount()
	if count < s.base.Cfg.Metrics.SyncServer.RoomScale.Small {
		s.data.Msg.Small ++
	} else if count >= s.base.Cfg.Metrics.SyncServer.RoomScale.Small && count < s.base.Cfg.Metrics.SyncServer.RoomScale.Middle {
		s.data.Msg.Middle++
	} else if count >= s.base.Cfg.Metrics.SyncServer.RoomScale.Middle && count < s.base.Cfg.Metrics.SyncServer.RoomScale.Large {
		s.data.Msg.Big++
	} else{
		s.data.Msg.Large++
	}
}


func GetSyncServerMetrics() *external.SyncServerMetrics {
	return syncServerExporter.getMetrics()
}

func SyncServerMsgInc(roomId string) {
	syncServerExporter.MsgInc(roomId)
}


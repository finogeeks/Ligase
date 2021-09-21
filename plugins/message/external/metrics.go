package external

// --- syncserver begin ---
const (
	ROOM_SCALE_LARGE = "大"
	ROOM_SCALE_BIG = "中"
	ROOM_SCALE_MIDDLE = "小"
	ROOM_SCALE_SMALL = "单聊"
)

type GetSyncServerMetricsRequest struct {
	Inst 	string 		`json:"inst"`
}

// 房间规模
type RoomScale struct {
	// 超大-大-中-小
	Label string `json:"label"`
	// 数量
	Count int64  `json:"count"`
	// 消息数量
	MsgCount int64 `json:"msg_count"`
}

type RoomScaleMetrics struct {
	Large 	RoomScale `json:"large"`
	Big   	RoomScale `json:"big"`
	Middle 	RoomScale `json:"middle"`
	Small 	RoomScale `json:"small"`
}

type SyncServerMetrics struct {
	// 实例ID
	Instance int   `json:"instance"`
	// 房间规模数据
	RoomScale RoomScaleMetrics `json:"room_scale"`
	// 消息总量
	MsgCount int64 `json:"msg_count"`
}
// --- syncserver end ---

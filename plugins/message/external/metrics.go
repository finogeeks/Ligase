package external

// --- syncserver begin ---

type GetSyncServerMetricsRequest struct {
	Inst 	string 		`json:"inst"`
}

// 房间规模
type RoomScale struct {
	// 超大-大-中-小
	Label string `json:"label"`
	// 数量
	Count int64  `json:"count"`
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
}
// --- syncserver end ---

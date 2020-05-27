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

package publicroomsapi

import (
	"context"

	"github.com/finogeeks/ligase/model/publicroomstypes"
)

type QueryPublicRoomsRequest struct {
	// Offset int64  `json:"offset"`
	Limit  int64  `json:"limit"`
	Since  string `json:"since"`
	Filter string `json:"filter"`
}

type QueryPublicRoomsResponse struct {
	Chunk     []publicroomstypes.PublicRoom `json:"chunk"`
	NextBatch string                        `json:"next_batch,omitempty"`
	PrevBatch string                        `json:"prev_batch,omitempty"`
	Estimate  int64                         `json:"total_room_count_estimate,omitempty"`
}

type PublicRoomsRpcRequest struct {
	QueryPublicRooms *QueryPublicRoomsRequest `json:"qry_public_rooms,omitempty"`
	Reply            string
}

type PublicRoomsQueryAPI interface {
	QueryPublicRooms(
		ctx context.Context,
		request *QueryPublicRoomsRequest,
		response *QueryPublicRoomsResponse,
	) error
}

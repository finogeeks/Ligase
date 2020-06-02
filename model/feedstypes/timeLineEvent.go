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

package feedstypes

import (
	"github.com/finogeeks/ligase/model/syncapitypes"
)

type TimeLineEvent struct {
	Offset int64
	Ev     *syncapitypes.UserTimeLineStream
}

func (se *TimeLineEvent) GetOffset() int64 {
	return se.Offset
}

func (se *TimeLineEvent) GetEv() *syncapitypes.UserTimeLineStream {
	return se.Ev
}

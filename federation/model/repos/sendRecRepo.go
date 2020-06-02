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

package repos

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/finogeeks/ligase/federation/storage/model"
	"github.com/finogeeks/ligase/model/service"
	"github.com/finogeeks/ligase/skunkworks/log"
)

type RecordItem struct {
	RoomID string
	Domain string

	EventID      string
	SendTimes    int32
	PendingSize  int32
	DomainOffset int64
}

type SendRecRepo struct {
	db    model.FederationDatabase
	cache service.Cache

	repo   sync.Map
	synced sync.Map
}

func NewSendRecRepo(
	db model.FederationDatabase,
	cache service.Cache,
) *SendRecRepo {
	ret := &SendRecRepo{
		db:    db,
		cache: cache,
	}
	return ret
}

func (r *SendRecRepo) LoadRooms() error {
	ctx := context.TODO()
	roomIDs, domains, _, _, _, _, _, err := r.db.SelectPendingSendRecord(ctx)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(roomIDs))
	for i := 0; i < len(roomIDs); i++ {
		keys = append(keys, roomIDs[i]+"|"+domains[i])
	}
	err = r.cache.AddFedPendingRooms(keys)
	if err != nil {
		return err
	}

	return nil
}

func (r *SendRecRepo) GetPenddingRooms(ctx context.Context, partition int32, count int) ([]string, []string) {
	keys, err := r.cache.QryFedPendingRooms()
	if err != nil {
		log.Errorf("GetPendingRooms error %v", err)
		return nil, nil
	}
	roomIDs := make([]string, 0, len(keys))
	domains := make([]string, 0, len(keys))
	for _, v := range keys {
		ss := strings.Split(v, "|")
		roomIDs = append(roomIDs, ss[0])
		domains = append(domains, ss[1])
	}

	return roomIDs, domains
}

func (r *SendRecRepo) GenerateFakePartition(ctx context.Context) int32 {
	p, err := r.cache.GetFakePartition("fedsend")
	if err != nil {
		log.Errorf("SendRecRepo generation fake partition error %v", err)
		return 0
	}
	return p
}

func (r *SendRecRepo) AssignRoomPartition(ctx context.Context, roomID, domain string, partition int32) (*RecordItem, bool) {
	err := r.cache.AssignFedSendRecPartition(roomID, domain, partition)
	if err != nil {
		log.Warnf("cache assign partition error %s %s %v", roomID, domain, err)
		return nil, false
	}
	recItem := r.GetRec(ctx, roomID, domain)
	if recItem == nil {
		r.cache.UnassignFedSendRecPartition(roomID, domain)
		log.Errorf("cache get rec item error %s %s", roomID, domain)
		return nil, false
	}
	return recItem, true
}

func (r *SendRecRepo) UnassignRoomPartition(ctx context.Context, roomID, domain string) error {
	err := r.cache.UnassignFedSendRecPartition(roomID, domain)
	r.repo.Delete(roomID + ":" + domain)
	return err
}

func (r *SendRecRepo) ExpireRoomPartition(ctx context.Context, roomID, domain string) error {
	err := r.cache.ExpireFedSendRecPartition(roomID, domain)
	return err
}

func (r *SendRecRepo) GetRec(ctx context.Context, roomID, domain string) *RecordItem {
	key := roomID + ":" + domain
	v, ok := r.repo.Load(key)
	if ok {
		return v.(*RecordItem)
	}
	recItem := &RecordItem{
		RoomID: roomID,
		Domain: domain,
	}
	eventID, sendTimes, pendingSize, domainOffset, err := r.cache.GetFedSendRec(roomID, domain)
	if err == nil {
		recItem.EventID = eventID
		recItem.SendTimes = sendTimes
		recItem.PendingSize = pendingSize
		recItem.DomainOffset = domainOffset
		v, _ = r.repo.LoadOrStore(roomID+":"+domain, recItem)
		return v.(*RecordItem)
	}
	eventID, sendTimes, pendingSize, domainOffset, err = r.db.SelectSendRecord(ctx, roomID, domain)
	if err == nil {
		r.cache.StoreFedSendRec(roomID, domain, eventID, sendTimes, pendingSize, domainOffset)
		recItem.EventID = eventID
		recItem.SendTimes = sendTimes
		recItem.PendingSize = pendingSize
		recItem.DomainOffset = domainOffset
		v, _ = r.repo.LoadOrStore(roomID+":"+domain, recItem)
		return v.(*RecordItem)
	}

	return nil
}

func (r *SendRecRepo) ReleaseSendRec(ctx context.Context, roomID, domain string) {
	r.repo.Delete(roomID + ":" + domain)
}

func (r *SendRecRepo) StoreRec(ctx context.Context, roomID, domain string, domainOffset int64) (*RecordItem, error) {
	recItem := &RecordItem{
		RoomID:       roomID,
		Domain:       domain,
		DomainOffset: domainOffset,
	}
	r.repo.Store(roomID+":"+domain, recItem)
	err := r.cache.StoreFedSendRec(roomID, domain, "", 0, 0, domainOffset)
	if err != nil {
		r.cache.FreeFedSendRec(roomID, domain)
	}
	if err := r.db.InsertSendRecord(context.TODO(), roomID, domain, domainOffset); err != nil {
		log.Errorf("fed-sender insert sending record roomID: %s domain: %s error: %v", roomID, domain, err)
		return nil, err
	}

	return recItem, nil
}

// NOTE: domainOffset 只有在插入的时候才会update，所以这里填收到的事件的domain_offset即可
func (r *SendRecRepo) IncrPendingSize(ctx context.Context, roomID, domain string, amt int, domainOffset int64) error {
	v, ok := r.repo.Load(roomID + ":" + domain)
	if ok {
		recItem := v.(*RecordItem)
		atomic.AddInt32(&recItem.PendingSize, int32(amt))
	}
	err := r.cache.IncrFedRoomPending(roomID, domain, amt)
	if err != nil {
		r.cache.FreeFedSendRec(roomID, domain)
	}
	err = r.db.UpdateSendRecordPendingSize(context.TODO(), roomID, domain, int32(amt), domainOffset)
	return err
}

func (r *SendRecRepo) UpdateDomainOffset(ctx context.Context, roomID, domain, eventID string, domainOffset int64, pendingSizeDecr int32) {
	v, ok := r.repo.Load(roomID + ":" + domain)
	if ok {
		recItem := v.(*RecordItem)
		recItem.SendTimes++
		recItem.DomainOffset = domainOffset
		recItem.EventID = eventID
		atomic.AddInt32(&recItem.PendingSize, -pendingSizeDecr)
	}
	err := r.cache.IncrFedRoomDomainOffset(roomID, domain, eventID, domainOffset, pendingSizeDecr)
	if err != nil {
		r.cache.FreeFedSendRec(roomID, domain)
	}
	err = r.db.UpdateSendRecordPendingSizeAndEventID(context.TODO(), roomID, domain, -pendingSizeDecr, eventID, domainOffset)
	if err != nil {
		return
	}
}

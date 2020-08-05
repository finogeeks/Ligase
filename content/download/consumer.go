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

package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/content/repos"
	"github.com/finogeeks/ligase/content/storage/model"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/federation/client"
	"github.com/finogeeks/ligase/model/mediatypes"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	"github.com/finogeeks/ligase/skunkworks/log"
)

const kDefaultWorkerCount = 10

type DownloadInfo struct {
	roomID  string
	userID  string
	eventID string
}

type DownloadConsumer struct {
	cfg        *config.Dendrite
	feddomains *common.FedDomains
	fedClient  *client.FedClientWrap
	db         model.ContentDatabase
	repo       *repos.DownloadStateRepo

	thumbnailMap map[string]DownloadInfo
	downloadMap  map[string]DownloadInfo

	curWatingList repos.WaitListSorted
	getMutex      sync.Mutex

	maxWorkerCount int32
	curWorkerCount int32

	httpCli *http.Client
}

func NewConsumer(
	cfg *config.Dendrite,
	feddomains *common.FedDomains,
	fedClient *client.FedClientWrap,
	db model.ContentDatabase,
	repo *repos.DownloadStateRepo,
) *DownloadConsumer {
	workerCount := kDefaultWorkerCount
	val, ok := common.GetTransportMultiplexer().GetChannel(
		cfg.Kafka.Consumer.DownloadMedia.Underlying,
		cfg.Kafka.Consumer.DownloadMedia.Name,
	)
	if ok {
		channel := val.(core.IChannel)
		s := &DownloadConsumer{
			cfg:            cfg,
			feddomains:     feddomains,
			fedClient:      fedClient,
			db:             db,
			repo:           repo,
			thumbnailMap:   make(map[string]DownloadInfo),
			downloadMap:    make(map[string]DownloadInfo),
			maxWorkerCount: int32(workerCount),
			httpCli: &http.Client{
				Transport: &http.Transport{
					DialContext: (&net.Dialer{
						Timeout: time.Second * 15,
					}).DialContext,
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					IdleConnTimeout:     time.Second * 90,
				},
			},
		}
		channel.SetHandler(s)

		return s
	}

	return nil
}

func (p *DownloadConsumer) Start() error {
	roomIDs, eventIDs, events, err := p.db.SelectMediaDownload(context.TODO())
	if err != nil {
		return err
	}

	for i := 0; i < len(roomIDs); i++ {
		var ev gomatrixserverlib.Event
		err := json.Unmarshal([]byte(events[i]), &ev)
		if err != nil {
			log.Errorf("DownloadConsumer event unmarshal error: %v, data: %v", err, events[i])
			continue
		}

		domain, url, thumbnailUrl := p.parseEv(&ev)
		if common.CheckValidDomain(domain, config.GetConfig().Matrix.ServerName) {
			continue
		}

		if url == "" || thumbnailUrl == "" {
			continue
		}

		roomID := ev.RoomID()
		userID := ev.Sender()
		if url != "" {
			_, netdiskID := p.splitMxc(url)
			p.pushDownload(roomID, userID, eventIDs[i], domain, netdiskID)
		}
		if thumbnailUrl != "" {
			_, netdiskID := p.splitMxc(thumbnailUrl)
			p.pushThumbnail(roomID, userID, eventIDs[i], domain, netdiskID)
		}
	}
	return nil
}

func (p *DownloadConsumer) startWorkerIfNeed() {
	p.getMutex.Lock()
	defer p.getMutex.Unlock()
	if p.curWorkerCount >= p.maxWorkerCount {
		return
	}
	p.curWorkerCount++
	go p.workerProcessor()
}

func (p *DownloadConsumer) workerProcessor() {
	defer func() {
		if e := recover(); e != nil {
			go p.workerProcessor()
		}
	}()
	span, ctx := common.StartSobSomSpan(context.Background(), "DownloadConsumer.workerProcessor")
	defer span.Finish()
	for {
		info, key, thumbnail, ok := func() (info DownloadInfo, key string, thumbnail, ok bool) {
			p.getMutex.Lock()
			defer p.getMutex.Unlock()
			info, key, thumbnail, ok = p.getOne()
			if !ok {
				p.curWorkerCount--
			}
			return
		}()
		if !ok {
			break
		}
		domain, netdiskID, _ := p.repo.SplitKey(key)
		err := p.download(ctx, info.userID, domain, netdiskID, thumbnail)
		if err != nil {
			p.repo.RemoveDownloading(key)
			if info.roomID != "" && info.eventID != "" {
				if thumbnail {
					p.pushThumbnail(info.roomID, info.userID, info.eventID, domain, netdiskID)
				} else {
					p.pushDownload(info.roomID, info.userID, info.eventID, domain, netdiskID)
				}
			}
		} else {
			p.repo.RemoveDownloading(key)
			if info.roomID != "" && info.eventID != "" {
				p.db.UpdateMediaDownload(ctx, info.roomID, info.eventID, true)
			}
		}
	}
}

func (p *DownloadConsumer) getOne() (info DownloadInfo, key string, thumbnail, ok bool) {
	if len(p.curWatingList) == 0 {
		waitingList := p.repo.GetWaitingList()
		p.curWatingList = waitingList
	}
	if len(p.curWatingList) > 0 {
		for len(p.curWatingList) > 0 {
			elem := p.curWatingList[0]
			p.curWatingList = p.curWatingList[1:]
			key = elem.GetKey()
			info, thumbnail, ok = p.getInQue(key, true)
			if ok {
				return
			}
		}
	}
	if len(p.thumbnailMap) > 0 {
		key := ""
		info := DownloadInfo{}
		for k, v := range p.thumbnailMap {
			key = k
			info = v
			break
		}
		delete(p.thumbnailMap, key)
		return info, key, true, true
	}
	if len(p.downloadMap) > 0 {
		key := ""
		info := DownloadInfo{}
		for k, v := range p.downloadMap {
			key = k
			info = v
			break
		}
		delete(p.downloadMap, key)
		return info, key, false, true
	}
	return DownloadInfo{}, "", false, false
}

func (p *DownloadConsumer) getFirstValidOne(list repos.WaitListSorted) (ret repos.WaitListSorted, info DownloadInfo, key string, thumbnail, ok bool) {
	for len(list) > 0 {
		elem := list[0]
		list = list[1:]
		key := elem.GetKey()
		info, thumbnail, ok := p.getInQue(key, true)
		if ok {
			return list, info, key, thumbnail, true
		}
	}
	return list, DownloadInfo{}, "", false, false
}

func (p *DownloadConsumer) getInQue(key string, isDelete bool) (info DownloadInfo, thumbnail, ok bool) {
	// don't lock
	if v, ok := p.thumbnailMap[key]; ok {
		if isDelete {
			delete(p.thumbnailMap, key)
		}
		return v, true, true
	}
	if v, ok := p.downloadMap[key]; ok {
		if isDelete {
			delete(p.downloadMap, key)
		}
		return v, false, true
	}

	return DownloadInfo{}, false, false
}

func (p *DownloadConsumer) pushThumbnail(roomID, userID, eventID, domain, netdiskID string) {
	key := p.repo.BuildKey(domain, netdiskID)
	p.repo.AddDownload(key)
	p.thumbnailMap[key] = DownloadInfo{roomID, userID, eventID}
	p.startWorkerIfNeed()
}

func (p *DownloadConsumer) pushDownload(roomID, userID, eventID, domain, netdiskID string) {
	key := p.repo.BuildKey(domain, netdiskID)
	p.repo.AddDownload(key)
	p.downloadMap[key] = DownloadInfo{roomID, userID, eventID}
	p.startWorkerIfNeed()
}

func (p *DownloadConsumer) OnMessage(ctx context.Context, topic string, partition int32, data []byte, rawMsg interface{}) {
	var ev gomatrixserverlib.Event
	err := json.Unmarshal(data, &ev)
	if err != nil {
		log.Errorf("DownloadConsumer event unmarshal error: %v, data: %v", err, data)
		return
	}

	domain, url, thumbnailUrl := p.parseEv(&ev)
	log.Infof("DownloadConsumer recvive %s %s %s", domain, url, thumbnailUrl)
	if common.CheckValidDomain(domain, config.GetConfig().Matrix.ServerName) {
		return
	}

	if url == "" && thumbnailUrl == "" {
		return
	}

	p.db.InsertMediaDownload(ctx, ev.RoomID(), ev.EventID(), string(data))
	roomID := ev.RoomID()
	userID := ev.Sender()
	eventID := ev.EventID()
	if url != "" {
		_, netdiskID := p.splitMxc(url)
		p.pushDownload(roomID, userID, eventID, domain, netdiskID)
	}
	if thumbnailUrl != "" {
		_, netdiskID := p.splitMxc(thumbnailUrl)
		p.pushThumbnail(roomID, userID, eventID, domain, netdiskID)
	}
}

// AddReq 不会保存到数据库
func (p *DownloadConsumer) AddReq(domain, netdiskID string) {
	p.pushDownload("", "@fakeUser:fakeDomain.com", "", domain, netdiskID)
}

func (p *DownloadConsumer) parseEv(ev *gomatrixserverlib.Event) (domain, url, thumbnailUrl string) {
	domain, _ = common.DomainFromID(ev.Sender())
	var content map[string]interface{}
	err := json.Unmarshal(ev.Content(), &content)
	if err != nil {
		log.Errorf("DownloadConsumer content wrong: %s", ev.Content())
		return
	}
	if !p.isMediaEv(content) {
		log.Errorf("DownloadConsumer is not media event: %s", ev.Content())
		return
	}

	v, ok := content["url"]
	if !ok {
		log.Errorf("DownloadConsumer netdisk url error: %s", ev.Content())
		return
	}
	url = v.(string)

	thumbnailUrl, _ = p.getThumbnalUrl(content)
	return
}

func (p *DownloadConsumer) download(ctx context.Context, userID, domain, netdiskID string, thumbnail bool) error {
	log.Infof("federation Download netdisk %s from remote %s", netdiskID, domain)
	destination, _ := p.feddomains.GetDomainHost(domain)
	info, err := p.fedClient.LookupMediaInfo(ctx, destination, netdiskID, userID)
	if err != nil {
		log.Errorf("federation Download get media info error: %v", err)
		return errors.New("federation Download get media info error:" + err.Error())
	}
	isEmote := false
	var contentParam mediatypes.MediaContentInfo
	err = json.Unmarshal([]byte(info.Content), &contentParam)
	if err == nil {
		isEmote = contentParam.IsEmote
	}
	err = p.fedClient.Download(ctx, destination, domain, netdiskID, "", "", "download", func(response *http.Response) error {
		if response == nil || response.Body == nil {
			log.Errorf("download fed netdisk response nil")
			return errors.New("download fed netdisk response nil")
		}
		defer func() {
			if response != nil && response.Body != nil {
				response.Body.Close()
			}
		}()

		reqUrl := p.cfg.Media.UploadUrl

		header := response.Header

		headStr, _ := json.Marshal(header)
		log.Infof("fed download, header for media upload request: %s, %s", string(headStr), header.Get("Content-Length"))

		newReq, err := http.NewRequest("POST", reqUrl, response.Body)
		if thumbnail {
			newReq.URL.Query().Set("thumbnail", "true")
		} else {
			newReq.URL.Query().Set("thumbnail", "false")
		}
		newReq.Header.Set("X-Consumer-Custom-ID", info.Owner)
		newReq.Header.Set("X-Consumer-NetDisk-ID", info.NetdiskID)
		newReq.Header.Set("Content-Type", header.Get("Content-Type"))

		q := newReq.URL.Query()
		q.Add("type", info.Type)
		q.Add("content", info.Content)
		if isEmote {
			q.Add("isemote", "true")
			q.Add("isfed", "true")
		}
		if thumbnail {
			q.Add("thumbnail", "true")
		} else {
			q.Add("thumbnail", "false")
		}

		// we must encode illegal chars
		newReq.URL.RawQuery = q.Encode()

		if err != nil {
			log.Errorf("fed download, upload to local NewRequest error: %v", err)
			return errors.New("fed download, upload to local NewRequest error:" + err.Error())
		}
		newReq.ContentLength, _ = strconv.ParseInt(header.Get("Content-Length"), 10, 0)

		headStr, _ = json.Marshal(newReq.Header)
		log.Infof("fed download, header for net disk request: %s", string(headStr))

		res, err := p.httpCli.Do(newReq)
		if err != nil {
			log.Errorf("fed download, upload file request error: %v", err)
			return errors.New("fed download, upload file request error:" + err.Error())
		}
		respData, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorf("upload netdisk read resp err: %v", err)
			return errors.New("upload netdisk read resp err: %v" + err.Error())
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			if res.StatusCode == http.StatusInternalServerError && bytes.Contains(respData, []byte("duplicate key error")) {
				log.Warnf("download remote netdiskID duplicate %s", netdiskID)
				return nil
			}
			log.Errorf("fed download, upload file response statusCode %d", res.StatusCode)
			var errInfo mediatypes.UploadError
			err = json.Unmarshal(respData, &errInfo)
			if err != nil {
				log.Errorf("fed download, upload file decode error: %v, data: %v", err, respData)
				return errors.New("fed download, upload file decode error: %v" + err.Error())
			}
			log.Errorf("fed download, upload file response %v", errInfo)
			return errors.New("fed download, upload file response" + string(respData))
		}
		if isEmote {
			var resp mediatypes.UploadEmoteResp
			data_, _ := ioutil.ReadAll(res.Body)
			err := json.Unmarshal(data_, &resp)
			if err != nil {
				log.Errorf("fed download, upload emote unmarhal resp error: %v, data: %v", err, respData)
				return errors.New("fed download, upload emote unmarhal resp error: %v" + err.Error())
			}
			log.Infof("fed download, upload emote succ resp:%+v", resp)
			return nil
		}
		var resp mediatypes.NetDiskResponse
		err = json.Unmarshal(respData, &resp)
		if err != nil {
			log.Errorf("fed download, upload file unmarhal resp error: %v, data: %v", err, respData)
			return errors.New("fed download, upload file unmarhal resp error: %v" + err.Error())
		}

		log.Info("MediaId: ", info.NetdiskID, " download in fed success")
		return nil
	})
	if err != nil {
		log.Errorf("federation Download [%s] error: %v", netdiskID, err)
	}
	return err
}

func (p *DownloadConsumer) getThumbnalUrl(content map[string]interface{}) (string, bool) {
	evInfo := content["info"]
	if infoMap, ok := evInfo.(map[string]interface{}); ok {
		if thumbnail, ok := infoMap["thumbnail_url"]; ok {
			if thumbnailUrl, ok := thumbnail.(string); ok && thumbnailUrl != "" {
				if !strings.HasPrefix(thumbnailUrl, "http") {
					return thumbnailUrl, true
				}
			}
		}
	}
	return "", false
}

func (p *DownloadConsumer) getMsgType(content map[string]interface{}) (string, bool) {
	value, exists := content["msgtype"]
	if !exists {
		return "", false
	}
	msgtype, ok := value.(string)
	if !ok {
		return "", false
	}
	return msgtype, true
}

func (p *DownloadConsumer) isMediaEv(content map[string]interface{}) bool {
	msgtype, ok := p.getMsgType(content)
	if !ok {
		return false
	}
	return msgtype == "m.image" || msgtype == "m.audio" || msgtype == "m.video" || msgtype == "m.file"
}

func (p *DownloadConsumer) splitMxc(s string) (domain, netdiskID string) {
	tmpUrl := strings.TrimPrefix(s, "mxc://")
	ss := strings.Split(tmpUrl, "/")
	if len(ss) != 2 {
		return "", s
	} else {
		return ss[0], ss[1]
	}
}

package routing

import (
	"encoding/json"
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/mediatypes"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

func (p *Processor) BuildBaseURL(host string, urlPath ...string) string {
	hostURL, _ := url.Parse(host)
	parts := []string{hostURL.Path}
	parts = append(parts, urlPath...)
	hostURL.Path = path.Join(parts...)
	if strings.HasSuffix(urlPath[len(urlPath)-1], "/") {
		hostURL.Path = hostURL.Path + "/"
	}
	query := hostURL.Query()
	hostURL.RawQuery = query.Encode()
	return hostURL.String()
}

func (p *Processor) uploadEmoteResp(rw http.ResponseWriter, res *http.Response, userID string) {
	if res.StatusCode != http.StatusOK {
		rw.WriteHeader(res.StatusCode)
		data, _ := ioutil.ReadAll(res.Body)
		rw.Write(data)
		return
	}
	var r mediatypes.UploadEmoteResp
	data_, _ := ioutil.ReadAll(res.Body)
	err := json.Unmarshal(data_, &r)
	if err != nil {
		log.Errorf("user:%s upload emote error:%v",  userID, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return
	}
	domain, _ := common.DomainFromID(userID)
	resp := mediatypes.UploadEmoteResp{
		NetdiskID: 	fmt.Sprintf(contentUri, domain, r.NetdiskID),
		FileType:	r.FileType,
		Total:		r.Total,
		GroupID:	r.GroupID,
		Finish:		r.Finish,
		WaitId:		r.WaitId,
		Emotes:		[]mediatypes.EmoteItem{},
	}
	for _, emote := range r.Emotes {
		resp.Emotes = append(resp.Emotes, mediatypes.EmoteItem{
			NetdiskID: fmt.Sprintf(contentUri, domain, emote.NetdiskID),
			FileType:  emote.FileType,
			GroupID:   emote.GroupID,
		})
	}
	data, _ := json.Marshal(resp)
	rw.WriteHeader(http.StatusOK)
	rw.Write(data)
}

func (p *Processor) WaitEmote(rw http.ResponseWriter, req *http.Request, device *authtypes.Device) {
	reqUrl := p.BuildBaseURL(p.cfg.Media.NetdiskUrl, "wait", "emote")
	reqUrl = p.buildUrl(req, reqUrl)
	res, err := p.httpRequest(device.UserID, req.Method, reqUrl, req)
	if err != nil {
		log.Errorf("user:%s wait emote url:%s err:%v", device.UserID, reqUrl, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return
	}
	p.uploadEmoteResp(rw, res, device.UserID)
}

func (p *Processor) CheckEmote(rw http.ResponseWriter, req *http.Request, device *authtypes.Device){
	vars := mux.Vars(req)
	domain := vars["serverName"]
	mediaID := vars["mediaId"]
	if !common.CheckValidDomain(domain,p.cfg.Matrix.ServerName){
		p.repo.Wait(req.Context(), domain, mediaID)
	}
	reqUrl := p.BuildBaseURL(p.cfg.Media.NetdiskUrl, "check", "emote", mediaID)
	log.Infof("forwardproxy CheckEmote url %s", reqUrl)
	req.Header.Add("X-Consumer-Custom-ID", device.UserID)
	p.forwardProxy(rw, req, reqUrl, []byte{}, "check emote")
}

func (p *Processor) FavoriteEmote(rw http.ResponseWriter, req *http.Request, device *authtypes.Device){
	vars := mux.Vars(req)
	domain := vars["serverName"]
	mediaID := vars["mediaId"]
	if !common.CheckValidDomain(domain,p.cfg.Matrix.ServerName){
		p.repo.Wait(req.Context(), domain, mediaID)
	}
	reqUrl := p.BuildBaseURL(p.cfg.Media.NetdiskUrl, "favorite", "emote", mediaID)
	log.Infof("forwardproxy FavoriteEmote url %s", reqUrl)
	req.Header.Add("X-Consumer-Custom-ID", device.UserID)
	p.forwardProxy(rw, req, reqUrl, []byte{}, "favorite emote")
}

func (p *Processor) FavoriteFileEmote(rw http.ResponseWriter, req *http.Request, device *authtypes.Device){
	vars := mux.Vars(req)
	domain := vars["serverName"]
	mediaID := vars["mediaId"]
	if !common.CheckValidDomain(domain,p.cfg.Matrix.ServerName){
		p.repo.Wait(req.Context(), domain, mediaID)
	}
	reqUrl := p.BuildBaseURL(p.cfg.Media.NetdiskUrl, "favorite", "fileemote", mediaID)
	log.Infof("forwardproxy FavoriteFileEmote url %s", reqUrl)
	req.Header.Add("X-Consumer-Custom-ID", device.UserID)
	p.forwardProxy(rw, req, reqUrl, []byte{}, "favorite fileemote")
}

func (p *Processor) ListEmote(rw http.ResponseWriter, req *http.Request, device *authtypes.Device){
	reqUrl := p.BuildBaseURL(p.cfg.Media.NetdiskUrl, "list", "emote")
	res, err := p.httpRequest(device.UserID, req.Method, reqUrl, req)
	if err != nil {
		log.Errorf("user:%s list emote err:%v", device.UserID, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return
	}
	resp := mediatypes.ListEmoteItem{}
	data_, _ := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(data_, &resp)
	if err != nil {
		log.Errorf("user:%s list emote err:%v", device.UserID, err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Internal Server Error. " + err.Error()))
		return
	}
	domain, _ := common.DomainFromID(device.UserID)
	for idx, item := range resp.Emotes {
		resp.Emotes[idx].NetdiskID = fmt.Sprintf(contentUri, domain, item.NetdiskID)
	}
	data, _ := json.Marshal(resp)
	rw.WriteHeader(http.StatusOK)
	rw.Write(data)
}


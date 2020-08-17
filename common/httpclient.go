package common

import (
	"github.com/go-resty/resty/v2"
	"net"
	"net/http"
	"time"
)

//default headers
var defaultHeaders = map[string]string{
	"Content-Type": "application/json",
}

type HttpClient struct {
	client *resty.Client
	headers map[string]string
}

//set transport
func createTransport(localAddr net.Addr) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 15 * time.Second,
	}
	if localAddr != nil {
		dialer.LocalAddr = localAddr
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   200,
	}
}

func NewHttpClient() *HttpClient{
	return &HttpClient{
		client: resty.NewWithClient(&http.Client{
			//default transport
			Transport: createTransport(nil),
			//default timeout
			Timeout: 15 *time.Second,
		}),
		headers: defaultHeaders,
	}
}

//set headers
func (h *HttpClient) SetHeaders(headers map[string]string) {
	h.headers = headers
	if h.headers == nil {
		h.headers = defaultHeaders
	}
}

//set timeout
func (h *HttpClient) SetTimeout(timeout int64){
	h.client.SetTimeout(time.Duration(timeout)*time.Second)
}

//set transport
func (h *HttpClient) SetTransport(transport http.RoundTripper){
	h.client.SetTransport(transport)
}

//set retry
func (h *HttpClient) SetRetry(count int, waitTime int64, maxWaitTime int64){
	//retry count
	h.client.SetRetryCount(count).
	//retry wait time
	SetRetryWaitTime(time.Duration(waitTime)*time.Second).
	//max retry wait time
	SetRetryMaxWaitTime(time.Duration(maxWaitTime)*time.Second)
}

func (h *HttpClient) Post(url string, payload []byte) (*resty.Response, error) {
	resp, err := h.client.R().
		SetHeaders(h.headers).
		SetBody(payload).
		Post(url)
	return resp, err
}

func (h *HttpClient) Get(url string) (*resty.Response, error) {
	return h.client.R().
		SetHeaders(h.headers).
		Get(url)
}

func (h *HttpClient) Put(url string, payload []byte) (*resty.Response, error) {
	return h.client.R().
		SetHeaders(h.headers).
		SetBody(payload).
		Put(url)
}

func (h *HttpClient) Delete(url string) (*resty.Response, error) {
	return h.client.R().
		SetHeaders(h.headers).
		Delete(url)
}

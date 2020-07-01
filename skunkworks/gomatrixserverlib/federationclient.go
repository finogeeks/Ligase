package gomatrixserverlib

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/crypto/ed25519"
)

// A FederationClient is a matrix federation client that adds
// "Authorization: X-Matrix" headers to requests that need ed25519 signatures
type FederationClient struct {
	Client
	serverName       ServerName
	serverKeyID      KeyID
	serverPrivateKey ed25519.PrivateKey
	proto            string
}

// NewFederationClient makes a new FederationClient
func NewFederationClient(
	serverName ServerName, keyID KeyID, privateKey ed25519.PrivateKey, rootCA, certPem, keyPem string,
) *FederationClient {
	if rootCA == "" {
		return &FederationClient{
			Client:           *NewClient(),
			serverName:       serverName,
			serverKeyID:      keyID,
			serverPrivateKey: privateKey,
			proto:            "http",
		}
	}

	return &FederationClient{
		Client:           *NewHttpsClient(rootCA, certPem, keyPem),
		serverName:       serverName,
		serverKeyID:      keyID,
		serverPrivateKey: privateKey,
		proto:            "https",
	}
}

func (ac *FederationClient) doRequest(ctx context.Context, r FederationRequest, resBody interface{}) error {
	/*if err := r.Sign(ac.serverName, ac.serverKeyID, ac.serverPrivateKey); err != nil {
		return err
	}*/

	// do request without sign
	if r.fields.Origin != "" && r.fields.Origin != ac.serverName {
		return fmt.Errorf("gomatrixserverlib: the request is already signed by a different server")
	}
	r.fields.Origin = ac.serverName

	req, err := r.HTTPRequest(ac.proto)
	if err != nil {
		return err
	}

	return ac.Client.DoRequestAndParseResponse(ctx, req, resBody)
}

var federationPathPrefix = "/_matrix/federation/v1"

func (ac *FederationClient) SendEvent(
	ctx context.Context, destination ServerName, e interface{},
) (res RespSend, err error) {
	path := federationPathPrefix + "/send/event/"
	req := NewFederationRequest("PUT", destination, path)
	if err = req.SetContent(e); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, &res)
	return
}

// SendTransaction sends a transaction
func (ac *FederationClient) SendTransaction(
	ctx context.Context, t Transaction,
) (res RespSend, err error) {
	path := federationPathPrefix + "/send/" + string(t.TransactionID) + "/"
	req := NewFederationRequest("PUT", t.Destination, path)
	if err = req.SetContent(t); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, &res)
	return
}

// MakeJoin makes a join m.room.member event for a room on a remote matrix server.
// This is used to join a room the local server isn't a member of.
// We need to query a remote server because if we aren't in the room we don't
// know what to use for the "prev_events" in the join event.
// The remote server should return us a m.room.member event for our local user
// with the "prev_events" filled out.
// If this successfully returns an acceptable event we will sign it with our
// server's key and pass it to SendJoin.
// See https://matrix.org/docs/spec/server_server/unstable.html#joining-rooms
func (ac *FederationClient) MakeJoin(
	ctx context.Context, s ServerName, roomID, userID string, ver []string,
) (res RespMakeJoin, err error) {
	path := federationPathPrefix + "/make_join/" +
		url.PathEscape(roomID) + "/" +
		url.PathEscape(userID)
	for i, v := range ver {
		if i == 0 {
			path += "?v=" + url.PathEscape(v)
		} else {
			path += "&v=" + url.PathEscape(v)
		}
	}
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

// SendJoin sends a join m.room.member event obtained using MakeJoin via a
// remote matrix server.
// This is used to join a room the local server isn't a member of.
// See https://matrix.org/docs/spec/server_server/unstable.html#joining-rooms
func (ac *FederationClient) SendJoin(
	ctx context.Context, s ServerName, roomID, eventID string, event Event,
) (res RespSendJoin, err error) {
	path := federationPathPrefix + "/send_join/" +
		url.PathEscape(roomID) + "/" +
		url.PathEscape(eventID)
	req := NewFederationRequest("PUT", s, path)
	if err = req.SetContent(event); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) MakeLeave(
	ctx context.Context, s ServerName, roomID, userID string,
) (res RespMakeLeave, err error) {
	path := federationPathPrefix + "/make_leave/" +
		url.PathEscape(roomID) + "/" +
		url.PathEscape(userID)
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) SendLeave(
	ctx context.Context, s ServerName, roomID, eventID string, event Event,
) (res RespSendLeave, err error) {
	path := federationPathPrefix + "/send_leave/" +
		url.PathEscape(roomID) + "/" +
		url.PathEscape(eventID)
	req := NewFederationRequest("PUT", s, path)
	if err = req.SetContent(event); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, &res)
	return
}

// SendInvite sends an invite m.room.member event to an invited server to be
// signed by it. This is used to invite a user that is not on the local server.
func (ac *FederationClient) SendInvite(
	ctx context.Context, s ServerName, event Event,
) (res RespInvite, err error) {
	path := federationPathPrefix + "/invite/" +
		url.PathEscape(event.RoomID()) + "/" +
		url.PathEscape(event.EventID())
	req := NewFederationRequest("PUT", s, path)
	if err = req.SetContent(event); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) SendInviteRaw(
	ctx context.Context, s ServerName, roomID, eventID string, event interface{},
) (res RespInvite, err error) {
	path := federationPathPrefix + "/invite/" +
		url.PathEscape(roomID) + "/" +
		url.PathEscape(eventID)
	req := NewFederationRequest("PUT", s, path)
	if err = req.SetContent(event); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, &res)
	return
}

// ExchangeThirdPartyInvite sends the builder of a m.room.member event of
// "invite" membership derived from a response from invites sent by an identity
// server.
// This is used to exchange a m.room.third_party_invite event for a m.room.member
// one in a room the local server isn't a member of.
func (ac *FederationClient) ExchangeThirdPartyInvite(
	ctx context.Context, s ServerName, builder EventBuilder,
) (err error) {
	path := federationPathPrefix + "/exchange_third_party_invite/" +
		url.PathEscape(builder.RoomID)
	req := NewFederationRequest("PUT", s, path)
	if err = req.SetContent(builder); err != nil {
		return
	}
	err = ac.doRequest(ctx, req, nil)
	return
}

// LookupState retrieves the room state for a room at an event from a
// remote matrix server as full matrix events.
func (ac *FederationClient) LookupState(
	ctx context.Context, s ServerName, roomID, eventID string,
) (res RespState, err error) {
	path := federationPathPrefix + "/state/" +
		url.PathEscape(roomID) +
		"/?event_id=" +
		url.QueryEscape(eventID)
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

// LookupStateIDs retrieves the room state for a room at an event from a
// remote matrix server as lists of matrix event IDs.
func (ac *FederationClient) LookupStateIDs(
	ctx context.Context, s ServerName, roomID, eventID string,
) (res RespStateIDs, err error) {
	path := federationPathPrefix + "/state_ids/" +
		url.PathEscape(roomID) +
		"/?event_id=" +
		url.QueryEscape(eventID)
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

// LookupRoomAlias looks up a room alias hosted on the remote server.
// The domain part of the roomAlias must match the name of the server it is
// being looked up on.
// If the room alias doesn't exist on the remote server then a 404 gomatrix.HTTPError
// is returned.
func (ac *FederationClient) LookupRoomAlias(
	ctx context.Context, s ServerName, roomAlias string,
) (res RespDirectory, err error) {
	path := federationPathPrefix + "/query/directory?room_alias=" +
		url.QueryEscape(roomAlias)
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

type BackfillRequest struct {
	EventID string `json:"event_id"`
	Limit   int    `json:"limit"`
	RoomID  string `json:"room_id"`
	Dir     string `json:"dir"`
	Domain  string `json:"domain"`
	Origin  string `json:"origin"`
}

type BackfillResponse struct {
	Error          string    `json:"error"`
	Origin         string    `json:"origin"`
	OriginServerTs Timestamp `json:"origin_server_ts"`
	PDUs           []Event   `json:"pdus"`
}

// Backfill asks a homeserver for events early enough for them to not be in the
// local database.
// See https://matrix.org/docs/spec/server_server/unstable.html#get-matrix-federation-v1-backfill-roomid
func (ac *FederationClient) Backfill(
	ctx context.Context, s ServerName, domain, roomID string, limit int, eventIDs []string, dir string,
) (res BackfillResponse, err error) {
	// Encode the room ID so it won't interfer with the path.
	roomID = url.PathEscape(roomID)

	// Parse the limit into a string so that we can include it in the URL's query.
	limitStr := strconv.Itoa(limit)

	// Define the URL's query.
	query := url.Values{}
	query["v"] = eventIDs
	query.Set("domain", domain)
	query.Set("limit", limitStr)
	query.Set("dir", dir)

	// Use the url.URL structure to easily generate the request's URI (path?query).
	u := url.URL{
		Path:     "/_matrix/federation/v1/backfill/" + roomID + "/",
		RawQuery: query.Encode(),
	}
	path := u.RequestURI()

	// Send the request.
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) LookupDisplayname(
	ctx context.Context, s ServerName, userID string,
) (res RespDisplayname, err error) {
	path := federationPathPrefix + "/query/profile?user_id=" +
		url.PathEscape(userID) + "&field=displayname"
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) LookupAvatarURL(
	ctx context.Context, s ServerName, userID string,
) (res RespAvatarURL, err error) {
	path := federationPathPrefix + "/query/profile?user_id=" +
		url.PathEscape(userID) + "&field=avatar_url"
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) LookupProfile(
	ctx context.Context, s ServerName, userID string,
) (res RespProfile, err error) {
	path := federationPathPrefix + "/query/profile?user_id=" +
		url.PathEscape(userID) + "&field="
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) GetMissingEvents(
	ctx context.Context, s ServerName, roomID string, content *ReqGetMissingEventContent,
) (res RespGetMissingEvents, err error) {
	path := federationPathPrefix + "/get_missing_events/" + url.PathEscape(roomID)
	req := NewFederationRequest("POST", s, path)
	req.SetContent(*content)
	err = ac.doRequest(ctx, req, &res)
	return
}

// LookupDeviceKeys looks up a batch of keys which are the counterpart of request body
// It gives a key per device in request body
// If the device is in wrong val it won't appear in response body
func (ac *FederationClient) LookupDeviceKeys(
	ctx context.Context, s ServerName, content *QueryRequest,
) (res QueryResponse, err error) {
	path := federationPathPrefix + "/user/keys/query"
	req := NewFederationRequest("POST", "python:9999", path)
	req.SetContent(*content)
	err = ac.doRequest(ctx, req, &res)
	return
}

// LookupOneTimeKeys lookup a key for certain device
// which is used to encryption chatting session set up
func (ac *FederationClient) LookupOneTimeKeys(
	ctx context.Context, s ServerName, roomID string, content *ClaimRequest,
) (res ClaimResponse, err error) {
	path := federationPathPrefix + "/keys/claim"
	req := NewFederationRequest("GET", s, path)
	req.SetContent(*content)
	err = ac.doRequest(ctx, req, &res)
	return
}

// Download download media
// which is used to encryption chatting session set up
func (ac *FederationClient) Download(
	ctx context.Context, s ServerName, domain, mediaID, width, method, fileType string, cb func(response *http.Response) error,
) (err error) {
	path := federationPathPrefix + "/media/download/" + domain + "/" + mediaID + "/" + fileType
	req := NewFederationRequest("GET", s, path)

	/*if err := r.Sign(ac.serverName, ac.serverKeyID, ac.serverPrivateKey); err != nil {
		return err
	}*/

	// do request without sign
	if req.fields.Origin != "" && req.fields.Origin != ac.serverName {
		return fmt.Errorf("gomatrixserverlib: the request is already signed by a different server")
	}
	req.fields.Origin = ac.serverName

	r, err := req.HTTPRequest(ac.proto)
	if err != nil {
		return err
	}
	if r.Form == nil {
		r.Form = make(url.Values)
	}
	r.Form.Set("width", width)
	r.Form.Set("method", method)

	response, err := ac.Client.DoHTTPRequest(ctx, r, true)
	if response != nil {
		defer response.Body.Close() // nolint: errcheck
	}

	if cb != nil {
		err = cb(response)
	}

	return
}

func (ac *FederationClient) LookupMediaInfo(
	ctx context.Context, s ServerName, mediaID, userID string,
) (res RespMediaInfo, err error) {
	path := federationPathPrefix + "/media/info/" + mediaID + "/" + userID
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

func (ac *FederationClient) LookupUserInfo(
	ctx context.Context, s ServerName, userID string,
) (res RespUserInfo, err error) {
	path := federationPathPrefix + "/query/user_info?user_id=" + userID
	req := NewFederationRequest("GET", s, path)
	err = ac.doRequest(ctx, req, &res)
	return
}

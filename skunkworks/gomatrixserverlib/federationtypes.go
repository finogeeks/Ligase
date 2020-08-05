package gomatrixserverlib

import (
	"context"
	"strconv"
	//"encoding/json"
	"fmt"

	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	log "github.com/finogeeks/ligase/skunkworks/log"
)

// A RespSend is the content of a response to PUT /_matrix/federation/v1/send/{txnID}/
type RespSend struct {
	// Map of event ID to the result of processing that event.
	PDUs map[string]PDUResult `json:"pdus"`
}

// A PDUResult is the result of processing a matrix room event.
type PDUResult struct {
	// If not empty then this is a human readable description of a problem
	// encountered processing an event.
	Error string `json:"error,omitempty"`
}

// A RespStateIDs is the content of a response to GET /_matrix/federation/v1/state_ids/{roomID}/{eventID}
type RespStateIDs struct {
	// A list of state event IDs for the state of the room before the requested event.
	StateEventIDs []string `json:"pdu_ids"`
	// A list of event IDs needed to authenticate the state events.
	AuthEventIDs []string `json:"auth_chain_ids"`
}

// A RespState is the content of a response to GET /_matrix/federation/v1/state/{roomID}/{eventID}
type RespState struct {
	// A list of events giving the state of the room before the request event.
	StateEvents []Event `json:"pdus"`
	// A list of events needed to authenticate the state events.
	AuthEvents []Event `json:"auth_chain"`
}

// A RespEventAuth is the content of a response to GET /_matrix/federation/v1/event_auth/{roomID}/{eventID}
type RespEventAuth struct {
	// A list of events needed to authenticate the state events.
	AuthEvents []Event `json:"auth_chain"`
}

type MessageContent struct {
	Body    string `json:"body"`
	MsgType string `json:"msgtype"`
}

// Events combines the auth events and the state events and returns
// them in an order where every event comes after its auth events.
// Each event will only appear once in the output list.
// Returns an error if there are missing auth events or if there is
// a cycle in the auth events.
func (r RespState) Events() ([]Event, error) {
	eventsByID := map[string]*Event{}
	// Collect a map of event reference to event
	for i := range r.StateEvents {
		eventsByID[r.StateEvents[i].EventID()] = &r.StateEvents[i]
	}
	for i := range r.AuthEvents {
		eventsByID[r.AuthEvents[i].EventID()] = &r.AuthEvents[i]
	}

	//queued := map[*Event]bool{}
	outputted := map[*Event]bool{}
	var result []Event
	for _, event := range eventsByID {
		if outputted[event] {
			// If we've already written the event then we can skip it.
			continue
		}

		// The code below does a depth first scan through the auth events
		// looking for events that can be appended to the output.

		// We use an explicit stack rather than using recursion so
		// that we can check we aren't creating cycles.
		stack := []*Event{event}

		//LoopProcessTopOfStack:
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			// Check if we can output the top of the stack.
			// We can output it if we have outputted all of its auth_events.
			/*
				for _, ref := range top.AuthEvents() {
					authEvent := eventsByID[ref.EventID]
					if authEvent == nil {
						return nil, fmt.Errorf(
							"gomatrixserverlib: missing auth event with ID %q for event %q",
							ref.EventID, top.EventID(),
						)
					}
					if outputted[authEvent] {
						continue
					}
					if queued[authEvent] {
						return nil, fmt.Errorf(
							"gomatrixserverlib: auth event cycle for ID %q",
							ref.EventID,
						)
					}
					// If we haven't visited the auth event yet then we need to
					// process it before processing the event currently on top of
					// the stack.
					stack = append(stack, authEvent)
					queued[authEvent] = true
					continue LoopProcessTopOfStack
				}
			*/
			// If we've processed all the auth events for the event on top of
			// the stack then we can append it to the result and try processing
			// the item below it in the stack.
			result = append(result, *top)
			outputted[top] = true
			stack = stack[:len(stack)-1]
		}
	}

	return result, nil
}

// Check that a response to /state is valid.
func (r RespState) Check(ctx context.Context, keyRing JSONVerifier) error {
	fields := util.GetLogFields(ctx)
	var allEvents []Event
	for _, event := range r.AuthEvents {
		if event.StateKey() == nil {
			return fmt.Errorf("gomatrixserverlib: event %q does not have a state key", event.EventID())
		}
		allEvents = append(allEvents, event)
	}

	stateTuples := map[StateKeyTuple]bool{}
	for _, event := range r.StateEvents {
		if event.StateKey() == nil {
			return fmt.Errorf("gomatrixserverlib: event %q does not have a state key", event.EventID())
		}
		stateTuple := StateKeyTuple{event.Type(), *event.StateKey()}
		if stateTuples[stateTuple] {
			return fmt.Errorf(
				"gomatrixserverlib: duplicate state key tuple (%q, %q)",
				event.Type(), *event.StateKey(),
			)
		}
		stateTuples[stateTuple] = true
		allEvents = append(allEvents, event)
	}

	// Check if the events pass signature checks.
	log.Infow(fmt.Sprintf("Checking event signatures for %d events of room state", len(allEvents)), fields)
	if err := VerifyAllEventSignatures(ctx, allEvents, keyRing); err != nil {
		return err
	}

	eventsByID := map[string]*Event{}
	// Collect a map of event reference to event
	for i := range allEvents {
		eventsByID[allEvents[i].EventID()] = &allEvents[i]
	}

	// Check whether the events are allowed by the auth rules.
	for _, event := range allEvents {
		if err := checkAllowedByAuthEvents(event, eventsByID); err != nil {
			return err
		}
	}

	return nil
}

// A RespMakeJoin is the content of a response to GET /_matrix/federation/v1/make_join/{roomID}/{userID}
type RespMakeJoin struct {
	// An incomplete m.room.member event for a user on the requesting server
	// generated by the responding server.
	// See https://matrix.org/docs/spec/server_server/unstable.html#joining-rooms
	JoinEvent EventBuilder `json:"event"`
}

func (r *RespMakeJoin) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *RespMakeJoin) Decode(data []byte) error {
	return json.Unmarshal(data, r)
}

// A RespSendJoin is the content of a response to PUT /_matrix/federation/v1/send_join/{roomID}/{eventID}
// It has the same data as a response to /state, but in a slightly different wire format.
type RespSendJoin RespState

// MarshalJSON implements json.Marshaller
func (r RespSendJoin) MarshalJSON() ([]byte, error) {
	// SendJoinResponses contain the same data as a StateResponse but are
	// formatted slightly differently on the wire:
	//  1) The "pdus" field is renamed to "state".
	//  2) The object is placed as the second element of a two element list
	//     where the first element is the constant integer 200.
	//
	//
	// So a state response of:
	//
	//		{"pdus": x, "auth_chain": y}
	//
	// Becomes:
	//
	//      [200, {"state": x, "auth_chain": y}]
	//
	// (This protocol oddity is the result of a typo in the synapse matrix
	//  server, and is preserved to maintain compatibility.)

	return json.Marshal([]interface{}{200, respSendJoinFields(r)})
}

// UnmarshalJSON implements json.Unmarshaller
func (r *RespSendJoin) UnmarshalJSON(data []byte) error {
	var tuple []rawJSON
	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}
	if len(tuple) != 2 {
		return fmt.Errorf("gomatrixserverlib: invalid send join response, invalid length: %d != 2", len(tuple))
	}
	var fields respSendJoinFields
	if err := json.Unmarshal(tuple[1], &fields); err != nil {
		return err
	}
	*r = RespSendJoin(fields)
	return nil
}

func (r *RespSendJoin) Encode() ([]byte, error) {
	return r.MarshalJSON()
}

func (r *RespSendJoin) Decode(data []byte) error {
	return r.UnmarshalJSON(data)
}

type respSendJoinFields struct {
	StateEvents []Event `json:"state"`
	AuthEvents  []Event `json:"auth_chain"`
}

// Check that a response to /send_join is valid.
// This checks that it would be valid as a response to /state
// This also checks that the join event is allowed by the state.
func (r RespSendJoin) Check(ctx context.Context, keyRing JSONVerifier, joinEvent Event) error {
	// First check that the state is valid and that the events in the response
	// are correctly signed.
	//
	// The response to /send_join has the same data as a response to /state
	// and the checks for a response to /state also apply.
	if err := RespState(r).Check(ctx, keyRing); err != nil {
		return err
	}

	stateEventsByID := map[string]*Event{}
	authEvents := NewAuthEvents(nil)
	for i, event := range r.StateEvents {
		stateEventsByID[event.EventID()] = &r.StateEvents[i]
		if err := authEvents.AddEvent(&r.StateEvents[i]); err != nil {
			return err
		}
	}

	// Now check that the join event is valid against its auth events.
	if err := checkAllowedByAuthEvents(joinEvent, stateEventsByID); err != nil {
		return err
	}

	// Now check that the join event is valid against the supplied state.
	if err := Allowed(joinEvent, &authEvents); err != nil {
		return fmt.Errorf(
			"gomatrixserverlib: event with ID %q is not allowed by the supplied state: %s",
			joinEvent.EventID(), err.Error(),
		)

	}

	return nil
}

// A RespMakeLeave is the content of a response to GET /_matrix/federation/v1/make_leave/{roomID}/{userID}
type RespMakeLeave struct {
	// An incomplete m.room.member event for a user on the requesting server
	// generated by the responding server.
	// See https://matrix.org/docs/spec/server_server/unstable#leaving-rooms-rejecting-invites
	Event EventBuilder `json:"event"`
}

func (r *RespMakeLeave) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *RespMakeLeave) Decode(data []byte) error {
	return json.Unmarshal(data, r)
}

// A RespSendLeave is the content of a response to PUT /_matrix/federation/v1/send_leave/{roomID}/{eventID}
// It has the same data as a response to /state, but in a slightly different wire format.
type RespSendLeave struct {
	Code int
}

// MarshalJSON implements json.Marshaller
func (r RespSendLeave) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{r.Code, map[string]interface{}{}})
}

// UnmarshalJSON implements json.Unmarshaller
func (r *RespSendLeave) UnmarshalJSON(data []byte) error {
	var tuple []rawJSON
	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}
	if len(tuple) != 2 {
		return fmt.Errorf("gomatrixserverlib: invalid send leave response, invalid length: %d != 2", len(tuple))
	}
	code, err := strconv.Atoi(string(tuple[0]))
	if err != nil {
		return err
	}
	r.Code = code
	return nil
}

func (r *RespSendLeave) Encode() ([]byte, error) {
	return r.MarshalJSON()
}

func (r *RespSendLeave) Decode(data []byte) error {
	return r.UnmarshalJSON(data)
}

// A RespDirectory is the content of a response to GET  /_matrix/federation/v1/query/directory
// This is returned when looking up a room alias from a remote server.
// See https://matrix.org/docs/spec/server_server/unstable.html#directory
type RespDirectory struct {
	// The matrix room ID the room alias corresponds to.
	RoomID string `json:"room_id"`
	// A list of matrix servers that the directory server thinks could be used
	// to join the room. The joining server may need to try multiple servers
	// before it finds one that it can use to join the room.
	Servers []ServerName `json:"servers"`
}

func checkAllowedByAuthEvents(event Event, eventsByID map[string]*Event) error {
	/*
		authEvents := NewAuthEvents(nil)
		for _, authRef := range event.AuthEvents() {
			authEvent := eventsByID[authRef.EventID]
			if authEvent == nil {
				return fmt.Errorf(
					"gomatrixserverlib: missing auth event with ID %q for event %q",
					authRef.EventID, event.EventID(),
				)
			}
			if err := authEvents.AddEvent(authEvent); err != nil {
				return err
			}
		}
		if err := Allowed(event, &authEvents); err != nil {
			return fmt.Errorf(
				"gomatrixserverlib: event with ID %q is not allowed by its auth_events: %s",
				event.EventID(), err.Error(),
			)
		}
	*/
	return nil
}

// RespInvite is the content of a response to PUT /_matrix/federation/v1/invite/{roomID}/{eventID}
type RespInvite struct {
	Code int
	// The invite event signed by recipient server.
	Event Event
}

// MarshalJSON implements json.Marshaller
func (r RespInvite) MarshalJSON() ([]byte, error) {
	// The wire format of a RespInvite is slightly is sent as the second element
	// of a two element list where the first element is the constant integer 200.
	// (This protocol oddity is the result of a typo in the synapse matrix
	//  server, and is preserved to maintain compatibility.)
	return json.Marshal([]interface{}{r.Code, respInviteFields{r.Event}})
}

// UnmarshalJSON implements json.Unmarshaller
func (r *RespInvite) UnmarshalJSON(data []byte) error {
	var tuple []rawJSON
	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}
	if len(tuple) != 2 {
		return fmt.Errorf("gomatrixserverlib: invalid invite response, invalid length: %d != 2", len(tuple))
	}

	code, err := strconv.Atoi(string(tuple[0]))
	if err != nil {
		return err
	}
	r.Code = code

	var fields respInviteFields
	if err := json.Unmarshal(tuple[1], &fields); err != nil {
		return err
	}
	r.Event = fields.Event
	return nil
}

func (r *RespInvite) Encode() ([]byte, error) {
	return r.MarshalJSON()
}

func (r *RespInvite) Decode(data []byte) error {
	return r.UnmarshalJSON(data)
}

type respInviteFields struct {
	Event Event `json:"event"`
}

type RespDisplayname struct {
	DisplayName string `json:"displayname"`
}

type RespAvatarURL struct {
	AvatarURL string `json:"avatar_url"`
}

type RespProfile struct {
	AvatarURL   string `json:"avatar_url"`
	DisplayName string `json:"displayname"`
}

type RespUserInfo struct {
	UserName  string `json:"user_name"`
	JobNumber string `json:"job_number"`
	Mobile    string `json:"mobile"`
	Landline  string `json:"landline"`
	Email     string `json:"email"`
	State     int    `json:"state,omitempty"`
}

type ReqGetMissingEventContent struct {
	EarliestEvents []string `json:"earliest_events"`
	LatestEvents   []string `json:"latest_events"`
	Limit          int      `json:"limit"`
	MinDepth       int64    `json:"min_depth"`
}

type RespGetMissingEvents struct {
	// Missing events, arbritrary order.
	Events []Event `json:"events"`
}

// ClaimRequest structure
type ClaimRequest struct {
	Timeout     int64                        `json:"timeout"`
	ClaimDetail map[string]map[string]string `json:"one_time_keys"`
}

// QueryRequest structure
type QueryRequest struct {
	DeviceKeys map[string][]string `json:"device_keys"`
}

// QueryResponse structure
type QueryResponse struct {
	DeviceKeys map[string]map[string]DeviceKeysQuery `json:"device_keys"`
}

// DeviceKeysQuery structure
type DeviceKeysQuery struct {
	UserID    string                       `json:"user_id"`
	DeviceID  string                       `json:"device_id"`
	Algorithm []string                     `json:"algorithms"`
	Keys      map[string]string            `json:"keys"`
	Signature map[string]map[string]string `json:"signatures"`
	Unsigned  UnsignedDeviceInfo           `json:"unsigned"`
}

// UnsignedDeviceInfo structure
type UnsignedDeviceInfo struct {
	Info string `json:"device_display_name"`
}

// ClaimResponse structure
type ClaimResponse struct {
	Failures  map[string]interface{}                       `json:"failures"`
	ClaimBody map[string]map[string]map[string]interface{} `json:"one_time_keys"`
}

type RespMediaInfo struct {
	NetdiskID string `json:"netdiskID"`
	Owner     string `json:"owner"`
	Type      string `json:"type"`
	SpaceID   string `json:"spaceID"`
	Content   string `json:"content"`
}

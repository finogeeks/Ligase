// Copyright 2017 Vector Creations Ltd
// Copyright 2017 New Vector Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package routing

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/common/uid"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/plugins/message/internals"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"

	"github.com/finogeeks/ligase/clientapi/httputil"
	"github.com/finogeeks/ligase/common/jsonerror"
	util "github.com/finogeeks/ligase/skunkworks/gomatrixutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

const (
	minPasswordLength = 8   // http://matrix.org/docs/spec/client_server/r0.2.0.html#password-based
	maxPasswordLength = 512 // https://github.com/matrix-org/synapse/blob/v0.20.0/synapse/rest/client/v2_alpha/register.py#L161
	maxUsernameLength = 254 // http://matrix.org/speculator/spec/HEAD/intro.html#user-identifiers TODO account for domain
	sessionIDLength   = 24
)

// sessionsDict keeps track of completed auth stages for each session.
type sessionsDict struct {
	sessions map[string][]string
}

// GetCompletedStages returns the completed stages for a session.
func (d sessionsDict) GetCompletedStages(sessionID string) []string {
	if completedStages, ok := d.sessions[sessionID]; ok {
		return completedStages
	}
	// Ensure that a empty slice is returned and not nil. See #399.
	return make([]string, 0)
}

// AAddCompletedStage records that a session has completed an auth stage.
func (d *sessionsDict) AddCompletedStage(sessionID string, stage string) {
	d.sessions[sessionID] = append(d.GetCompletedStages(sessionID), stage)
}

func newSessionsDict() *sessionsDict {
	return &sessionsDict{
		sessions: make(map[string][]string),
	}
}

var (
	// TODO: Remove old sessions. Need to do so on a session-specific timeout.
	// sessions stores the completed flow stages for all sessions. Referenced using their sessionID.
	sessions           = newSessionsDict()
	validUsernameRegex = regexp.MustCompile(`^[0-9a-z_\-./]+$`)
)

// newUserInteractiveResponse will return a struct to be sent back to the client
// during registration.
func newUserInteractiveResponse(
	sessionID string,
	fs []external.AuthFlow,
	params map[string]interface{},
) *external.UserInteractiveResponse {
	return &external.UserInteractiveResponse{
		fs, sessions.GetCompletedStages(sessionID), params, sessionID,
	}
}

// http://matrix.org/speculator/spec/HEAD/client_server/unstable.html#post-matrix-client-unstable-register

// recaptchaResponse represents the HTTP response from a Google Recaptcha server
type recaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []int     `json:"error-codes"`
}

// validateUserName returns an error response if the username is invalid
func validateUserName(username string) (int, core.Coder) {
	// https://github.com/matrix-org/synapse/blob/v0.20.0/synapse/rest/client/v2_alpha/register.py#L161
	if len(username) > maxUsernameLength {
		return http.StatusBadRequest, jsonerror.BadJSON(fmt.Sprintf("'username' >%d characters", maxUsernameLength))
	} else if !validUsernameRegex.MatchString(username) {
		return http.StatusBadRequest, jsonerror.InvalidUsername("User ID can only contain characters a-z, 0-9, or '_-./'")
	} else if username[0] == '_' { // Regex checks its not a zero length string
		return http.StatusBadRequest, jsonerror.InvalidUsername("User ID can't start with a '_'")
	}
	return http.StatusOK, nil
}

// validatePassword returns an error response if the password is invalid
func validatePassword(password string) (int, core.Coder) {
	// https://github.com/matrix-org/synapse/blob/v0.20.0/synapse/rest/client/v2_alpha/register.py#L161
	if len(password) > maxPasswordLength {
		return http.StatusBadRequest, jsonerror.BadJSON(fmt.Sprintf("'password' >%d characters", maxPasswordLength))
	} else if len(password) > 0 && len(password) < minPasswordLength {
		return http.StatusBadRequest, jsonerror.WeakPassword(fmt.Sprintf("password too weak: min %d chars", minPasswordLength))
	}
	return http.StatusOK, nil
}

// validateRecaptcha returns an error response if the captcha response is invalid
func validateRecaptcha(
	cfg *config.Dendrite,
	response string,
	clientIp string,
) (int, core.Coder) {
	if !cfg.Matrix.RecaptchaEnabled {
		return http.StatusBadRequest, jsonerror.BadJSON("Captcha registration is disabled")
	}

	if response == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("Captcha response is required")
	}

	// Make a POST request to Google's API to check the captcha response
	resp, err := http.PostForm(cfg.Matrix.RecaptchaSiteVerifyAPI,
		url.Values{
			"secret":   {cfg.Matrix.RecaptchaPrivateKey},
			"response": {response},
			"remoteip": {clientIp},
		},
	)

	if err != nil {
		return http.StatusInternalServerError, jsonerror.BadJSON("Error in requesting validation of captcha response")
	}

	// Close the request once we're finishing reading from it
	defer resp.Body.Close() // nolint: errcheck

	// Grab the body of the response from the captcha server
	var r recaptchaResponse
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.BadJSON("Error in contacting captcha server" + err.Error())
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.BadJSON("Error in unmarshaling captcha server's response: " + err.Error())
	}

	// Check that we received a "success"
	if !r.Success {
		return http.StatusUnauthorized, jsonerror.BadJSON("Invalid captcha response. Please try again.")
	}
	return http.StatusOK, nil
}

// UsernameIsWithinApplicationServiceNamespace checks to see if a username falls
// within any of the namespaces of a given Application Service. If no
// Application Service is given, it will check to see if it matches any
// Application Service's namespace.
func UsernameIsWithinApplicationServiceNamespace(
	cfg *config.Dendrite,
	username string,
	appService *config.ApplicationService,
) bool {

	if appService != nil {
		// Loop through given Application Service's namespaces and see if any match
		for _, namespace := range appService.NamespaceMap["users"] {
			// AS namespaces are checked for validity in config
			namespace.RegexpObject, _ = regexp.Compile(namespace.Regex)
			if namespace.RegexpObject.MatchString(username) {
				return true
			}
		}
		return false
	}

	// Loop through all known Application Service's namespaces and see if any match
	for _, knownAppService := range cfg.Derived.ApplicationServices {
		for _, namespace := range knownAppService.NamespaceMap["users"] {
			// AS namespaces are checked for validity in config
			namespace.RegexpObject, _ = regexp.Compile(namespace.Regex)
			if namespace.RegexpObject.MatchString(username) {
				return true
			}
		}
	}
	return false
}

// UsernameMatchesMultipleExclusiveNamespaces will check if a given username matches
// more than one exclusive namespace. More than one is not allowed
func UsernameMatchesMultipleExclusiveNamespaces(
	cfg *config.Dendrite,
	username string,
) bool {
	// Check namespaces and see if more than one match
	matchCount := 0
	for _, appService := range cfg.Derived.ApplicationServices {
		for _, namespaceSlice := range appService.NamespaceMap {
			for _, namespace := range namespaceSlice {
				// Check if we have a match on this username
				namespace.RegexpObject, _ = regexp.Compile(namespace.Regex)
				if namespace.Exclusive && namespace.RegexpObject.MatchString(username) {
					matchCount++
				}
			}
		}
	}
	return matchCount > 1
}

// validateApplicationService checks if a provided application service token
// corresponds to one that is registered. If so, then it checks if the desired
// username is within that application service's namespace. As long as these
// two requirements are met, no error will be returned.
func validateApplicationService(
	cfg *config.Dendrite,
	req *external.PostRegisterRequest,
	username string,
) (string, int, core.Coder) {
	// Check if the token if the application service is valid with one we have
	// registered in the config.
	accessToken := req.AccessToken
	userID := username
	var matchedApplicationService *config.ApplicationService
	for _, appService := range cfg.Derived.ApplicationServices {
		if appService.ASToken == accessToken {
			matchedApplicationService = &appService
			break
		}
	}
	if matchedApplicationService == nil {
		log.Infof("invalid token Supplied access_token not match userID:%s", userID)
		return "", http.StatusUnauthorized, jsonerror.UnknownToken("Supplied access_token does not match any known application service")
	}

	// Ensure the desired username is within at least one of the application service's namespaces.
	if !UsernameIsWithinApplicationServiceNamespace(cfg, userID, matchedApplicationService) {
		// If we didn't find any matches, return M_EXCLUSIVE
		return "", http.StatusUnauthorized, jsonerror.ASExclusive(fmt.Sprintf(
			"Supplied username %s did not match any namespaces for application service ID: %s", username, matchedApplicationService.ID))
	}

	// Check this user does not fit multiple application service namespaces
	if UsernameMatchesMultipleExclusiveNamespaces(cfg, userID) {
		return "", http.StatusUnauthorized, jsonerror.ASExclusive(fmt.Sprintf(
			"Supplied username %s matches multiple exclusive application service namespaces. Only 1 match allowed", username))
	}

	// No errors, registration valid
	return matchedApplicationService.ID, 0, nil
}

// Register processes a /register request.
// http://matrix.org/speculator/spec/HEAD/client_server/unstable.html#post-matrix-client-unstable-register
func Register(
	ctx context.Context,
	req *external.PostRegisterRequest,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	cfg *config.Dendrite,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	kind := req.Kind
	if kind == "guest" {
		domain := req.Domain
		return handleGuestRegister(idg, cfg, domain)
	}

	// var r external.PostRegisterRequest
	// resErr := httputil.UnmarshalJSONRequest(req, &r)
	if req.Username == "" {
		return 400, jsonerror.NotJSON("The request body could not be decoded into valid JSON. ")
	}
	// if resErr != nil {
	// 	return *resErr
	// }

	// Retrieve or generate the sessionID
	sessionID := req.Auth.Session
	if sessionID == "" {
		// Generate a new, random session ID
		sessionID = util.RandomString(sessionIDLength)
	}

	// If no auth type is specified by the client, send back the list of available flows
	if req.Auth.Type == "" {
		return http.StatusUnauthorized, newUserInteractiveResponse(sessionID,
			cfg.Derived.Registration.Flows, cfg.Derived.Registration.Params)
	}

	_, _, err := gomatrixserverlib.SplitID('@', req.Username)
	if err != nil {
		return http.StatusBadRequest, jsonerror.InvalidUsername("User ID must be @localpart:domain")
	}

	// Squash username to all lowercase letters
	req.Username = strings.ToLower(req.Username)

	if code, err := validateUserName(req.Username); err != nil {
		return code, err
	}
	if code, err := validatePassword(req.Password); err != nil {
		return code, err
	}

	// Make sure normal user isn't registering under an exclusive application
	// service namespace. Skip this check if no app services are registered.
	if req.Auth.Type != "m.login.application_service" &&
		len(cfg.Derived.ApplicationServices) != 0 &&
		cfg.Derived.ExclusiveApplicationServicesUsernameRegexp.MatchString(req.Username) {
		return http.StatusBadRequest, jsonerror.ASExclusive("This username is reserved by an application service.")
	}

	fields := util.GetLogFields(ctx)
	fields = append(fields, log.KeysAndValues{
		"username", req.Username, "auth.type", req.Auth.Type, "session_id", req.Auth.Session,
	}...)
	log.Infow("Processing registration request", fields)

	return handleRegistrationFlow(ctx, req, sessionID, cfg, accountDB, deviceDB, idg)
}

func handleGuestRegister(idg *uid.UidGenerator, cfg *config.Dendrite, domain string) (int, core.Coder) {
	id, err := idg.Next()
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("Failed to generate uid")
	}

	if common.CheckValidDomain(domain, cfg.Matrix.ServerName) == false {
		return http.StatusBadRequest, jsonerror.BadJSON("domain not valid")
	}

	mac := strconv.FormatInt(id, 10)
	uid := fmt.Sprintf("@%s:%s", mac, domain)
	deviceID, deviceType, err := common.BuildDevice(idg, nil, true, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("Failed to generate device id")
	}

	token, err := common.BuildToken(cfg.Macaroon.Key, uid, domain, uid, mac, true, deviceID, deviceType, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("Failed to generate access token")
	}

	return http.StatusOK, &external.RegisterResponse{
		UserID:      uid,
		AccessToken: token,
		HomeServer:  domain,
		DeviceID:    deviceID,
	}
}

// handleRegistrationFlow will direct and complete registration flow stages
// that the client has requested.
func handleRegistrationFlow(
	ctx context.Context,
	req *external.PostRegisterRequest,
	sessionID string,
	cfg *config.Dendrite,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	// TODO: Shared secret registration (create new user scripts)
	// TODO: Enable registration config flag
	// TODO: Guest account upgrading

	// TODO: Handle loading of previous session parameters from database.
	// TODO: Handle mapping registrationRequest parameters into session parameters

	// TODO: email / msisdn auth types.

	if cfg.Matrix.RegistrationDisabled && req.Auth.Type != authtypes.LoginTypeSharedSecret {
		return http.StatusForbidden, &internals.RespMessage{Message: "Registration has been disabled"}
	}

	switch req.Auth.Type {
	case authtypes.LoginTypeRecaptcha:
		// Check given captcha response
		if code, err := validateRecaptcha(cfg, req.Auth.Response, req.RemoteAddr); err != nil {
			return code, err
		}

		// Add Recaptcha to the list of completed registration stages
		sessions.AddCompletedStage(sessionID, authtypes.LoginTypeRecaptcha)

	case authtypes.LoginTypeSharedSecret:
		// Check shared secret against config
		valid, err := isValidMacLogin(cfg, req.Username, req.Password, req.Admin, req.Auth.Mac)

		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		} else if !valid {
			return http.StatusForbidden, &internals.RespMessage{Message: "HMAC incorrect"}
		}

		// Add SharedSecret to the list of completed registration stages
		sessions.AddCompletedStage(sessionID, authtypes.LoginTypeSharedSecret)

	case authtypes.LoginTypeApplicationService:
		// Check Application Service register user request is valid.
		// The application service's ID is returned if so.
		appServiceID, code, err := validateApplicationService(cfg, req, req.Username)

		if err != nil {
			return code, err
		}

		// If no error, application service was successfully validated.
		// Don't need to worry about appending to registration stages as
		// application service registration is entirely separate.
		return completeRegistration(ctx, cfg, accountDB, deviceDB,
			req.Username, "", appServiceID, req.InitialDisplayName, idg)

	case authtypes.LoginTypeDummy:
		// there is nothing to do
		// Add Dummy to the list of completed registration stages
		sessions.AddCompletedStage(sessionID, authtypes.LoginTypeDummy)

	default:
		return http.StatusNotImplemented, jsonerror.Unknown("unknown/unimplemented auth type")
	}

	// Check if the user's registration flow has been completed successfully
	// A response with current registration flow and remaining available methods
	// will be returned if a flow has not been successfully completed yet
	return checkAndCompleteFlow(sessions.GetCompletedStages(sessionID),
		ctx, req, sessionID, cfg, accountDB, deviceDB, idg)
}

// checkAndCompleteFlow checks if a given registration flow is completed given
// a set of allowed flows. If so, registration is completed, otherwise a
// response with
func checkAndCompleteFlow(
	flow []string,
	ctx context.Context,
	r *external.PostRegisterRequest,
	sessionID string,
	cfg *config.Dendrite,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	if checkFlowCompleted(flow, cfg.Derived.Registration.Flows) {
		// This flow was completed, registration can continue
		return completeRegistration(ctx, cfg, accountDB, deviceDB,
			r.Username, r.Password, "", r.InitialDisplayName, idg)
	}

	// There are still more stages to complete.
	// Return the flows and those that have been completed.
	return http.StatusUnauthorized, newUserInteractiveResponse(sessionID,
		cfg.Derived.Registration.Flows, cfg.Derived.Registration.Params)
}

// LegacyRegister process register requests from the legacy v1 API
func LegacyRegister(
	ctx context.Context,
	req *external.LegacyRegisterRequest,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	cfg *config.Dendrite,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	// var r external.LegacyRegisterRequest
	// code, err := parseAndValidateLegacyLogin(req, &r)
	// if err != nil {
	// 	return code, err
	// }

	fields := util.GetLogFields(ctx)
	fields = append(fields, log.KeysAndValues{"username", req.Username, "auth.type", req.Type}...)
	log.Infow("Processing registration request", fields)

	if cfg.Matrix.RegistrationDisabled && req.Type != authtypes.LoginTypeSharedSecret {
		return http.StatusForbidden, &internals.RespMessage{Message: "Registration has been disabled"}
	}

	switch req.Type {
	case authtypes.LoginTypeSharedSecret:
		if cfg.Matrix.RegistrationSharedSecret == "" {
			return http.StatusBadRequest, &internals.RespMessage{Message: "Shared secret registration is disabled"}
		}

		valid, err := isValidMacLogin(cfg, req.Username, req.Password, req.Admin, req.Mac)
		if err != nil {
			return httputil.LogThenErrorCtx(ctx, err)
		}

		if !valid {
			return http.StatusForbidden, &internals.RespMessage{Message: "HMAC incorrect"}
		}

		return completeRegistration(ctx, cfg, accountDB, deviceDB, req.Username, req.Password, "", "", idg)
	case authtypes.LoginTypeDummy:
		// there is nothing to do
		return completeRegistration(ctx, cfg, accountDB, deviceDB, req.Username, req.Password, "", "", idg)
	default:
		return http.StatusNotImplemented, jsonerror.Unknown("unknown/unimplemented auth type")
	}
}

// parseAndValidateLegacyLogin parses the request into r and checks that the
// request is valid (e.g. valid user names, etc)
func parseAndValidateLegacyLogin(req *http.Request, r *external.LegacyRegisterRequest) (int, core.Coder) {
	// resErr := httputil.UnmarshalJSONRequest(req, &r)
	// if resErr != nil {
	// 	return http.StatusBadRequest, resErr
	// }

	// Squash username to all lowercase letters
	// r.Username = strings.ToLower(r.Username)

	if code, err := validateUserName(r.Username); err != nil {
		return code, err
	}
	if code, err := validatePassword(r.Password); err != nil {
		return code, err
	}

	// All registration requests must specify what auth they are using to perform this request
	if r.Type == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("invalid type")
	}

	return http.StatusOK, nil
}

func completeRegistration(
	ctx context.Context,
	cfg *config.Dendrite,
	accountDB model.AccountsDatabase,
	deviceDB model.DeviceDatabase,
	username, password, appServiceID string,
	displayName string,
	idg *uid.UidGenerator,
) (int, core.Coder) {
	if username == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("missing username")
	}
	// Blank passwords are only allowed by registered application services
	if password == "" && appServiceID == "" {
		return http.StatusBadRequest, jsonerror.BadJSON("missing password")
	}

	acc, err := accountDB.CreateAccount(ctx, username, password, appServiceID, displayName)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create account: " + err.Error())
	} else if acc == nil {
		return http.StatusBadRequest, jsonerror.UserInUse("Desired user ID is already taken.")
	}

	// // TODO: Use the device ID in the request.
	deviceID, deviceType, err := common.BuildDevice(idg, nil, true, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create device: " + err.Error())
	}

	domain, _ := common.DomainFromID(username)
	token, err := common.BuildToken(cfg.Macaroon.Key, username, domain, username, "", false, deviceID, deviceType, true)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("Failed to generate access token")
	}

	dev, err := deviceDB.CreateDevice(ctx, username, deviceID, deviceType, &displayName, true, nil, -1)
	if err != nil {
		return http.StatusInternalServerError, jsonerror.Unknown("failed to create device: " + err.Error())
	}

	return http.StatusOK, &external.RegisterResponse{
		UserID:      dev.UserID,
		AccessToken: token,
		HomeServer:  domain,
		DeviceID:    dev.ID,
	}
}

// Used for shared secret registration.
// Checks if the username, password and isAdmin flag matches the given mac.
func isValidMacLogin(
	cfg *config.Dendrite,
	username, password string,
	isAdmin bool,
	givenMac []byte,
) (bool, error) {
	sharedSecret := cfg.Matrix.RegistrationSharedSecret

	// Check that shared secret registration isn't disabled.
	if cfg.Matrix.RegistrationSharedSecret == "" {
		return false, errors.New("Shared secret registration is disabled")
	}

	// Double check that username/password don't contain the HMAC delimiters. We should have
	// already checked this.
	if strings.Contains(username, "\x00") {
		return false, errors.New("Username contains invalid character")
	}
	if strings.Contains(password, "\x00") {
		return false, errors.New("Password contains invalid character")
	}
	if sharedSecret == "" {
		return false, errors.New("Shared secret registration is disabled")
	}

	adminString := "notadmin"
	if isAdmin {
		adminString = "admin"
	}
	joined := strings.Join([]string{username, password, adminString}, "\x00")

	mac := hmac.New(sha1.New, []byte(sharedSecret))
	_, err := mac.Write([]byte(joined))
	if err != nil {
		return false, err
	}
	expectedMAC := mac.Sum(nil)

	return hmac.Equal(givenMac, expectedMAC), nil
}

// checkFlows checks a single completed flow against another required one. If
// one contains at least all of the stages that the other does, checkFlows
// returns true.
func checkFlows(
	completedStages []string,
	requiredStages []string,
) bool {
	// Create temporary slices so they originals will not be modified on sorting
	completed := make([]string, len(completedStages))
	required := make([]string, len(requiredStages))
	copy(completed, completedStages)
	copy(required, requiredStages)

	// Sort the slices for simple comparison
	sort.Slice(completed, func(i, j int) bool { return completed[i] < completed[j] })
	sort.Slice(required, func(i, j int) bool { return required[i] < required[j] })

	// Iterate through each slice, going to the next required slice only once
	// we've found a match.
	i, j := 0, 0
	for j < len(required) {
		// Exit if we've reached the end of our input without being able to
		// match all of the required stages.
		if i >= len(completed) {
			return false
		}

		// If we've found a stage we want, move on to the next required stage.
		if completed[i] == required[j] {
			j++
		}
		i++
	}
	return true
}

// checkFlowCompleted checks if a registration flow complies with any allowed flow
// dictated by the server. Order of stages does not matter. A user may complete
// extra stages as long as the required stages of at least one flow is met.
func checkFlowCompleted(
	flow []string,
	allowedFlows []external.AuthFlow,
) bool {
	// Iterate through possible flows to check whether any have been fully completed.
	for _, allowedFlow := range allowedFlows {
		if checkFlows(flow, allowedFlow.Stages) {
			return true
		}
	}
	return false
}

// RegisterAvailable checks if the username is already taken or invalid.
func RegisterAvailable() (int, core.Coder) {
	//此接口无用，不做特别处理

	return http.StatusOK, &external.GetRegisterAvailResponse{
		Available: true,
	}
}

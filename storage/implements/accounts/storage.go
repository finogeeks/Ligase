// Copyright 2017 Vector Creations Ltd
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

package accounts

import (
	"context"
	"database/sql"
	"time"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/core"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/model/dbtypes"
	log "github.com/finogeeks/ligase/skunkworks/log"
	mon "github.com/finogeeks/ligase/skunkworks/monitor/go-client/monitor"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
)

func init() {
	common.Register("accounts", NewDatabase)
}

// Database represents an account database
type Database struct {
	db          *sql.DB
	topic       string
	underlying  string
	accounts    accountsStatements
	profiles    profilesStatements
	accountData accountDataStatements
	filter      filterStatements
	tags        roomTagsStatements
	userInfo    userInfoStatements
	AsyncSave   bool

	qryDBGauge mon.LabeledGauge
}

// NewDatabase creates a new accounts and profiles database
//conf.Database.Account
func NewDatabase(driver, createAddr, address, underlying, topic string, useAsync bool) (interface{}, error) {
	acc := new(Database)
	var err error

	common.CreateDatabase(driver, createAddr, "dendrite_account")

	if acc.db, err = sql.Open(driver, address); err != nil {
		return nil, err
	}

	acc.db.SetMaxOpenConns(30)
	acc.db.SetMaxIdleConns(30)
	acc.db.SetConnMaxLifetime(time.Minute * 3)

	schemas := []string{acc.accounts.getSchema(), acc.profiles.getSchema(), acc.accountData.getSchema(), acc.filter.getSchema(), acc.tags.getSchema(), acc.userInfo.getSchema()}
	for _, sqlStr := range schemas {
		_, err := acc.db.Exec(sqlStr)
		if err != nil {
			return nil, err
		}
	}

	if err = acc.accounts.prepare(acc); err != nil {
		return nil, err
	}
	if err = acc.profiles.prepare(acc); err != nil {
		return nil, err
	}
	if err = acc.accountData.prepare(acc); err != nil {
		return nil, err
	}
	if err = acc.filter.prepare(acc); err != nil {
		return nil, err
	}
	if err = acc.tags.prepare(acc); err != nil {
		return nil, err
	}
	if err = acc.userInfo.prepare(acc); err != nil {
		return nil, err
	}

	acc.AsyncSave = useAsync
	acc.underlying = underlying
	acc.topic = topic

	return acc, nil
}

func (d *Database) SetGauge(qryDBGauge mon.LabeledGauge) {
	d.qryDBGauge = qryDBGauge
}

func (d *Database) RecoverCache() {
	span, ctx := common.StartSobSomSpan(context.Background(), "RecoverCache")
	defer span.Finish()
	err := d.profiles.recoverProfile(ctx)
	if err != nil {
		log.Errorf("profiles.recoverProfile error %v", err)
	}

	err = d.accountData.recoverAccountData(ctx)
	if err != nil {
		log.Errorf("accountData.recoverAccountData error %v", err)
	}

	err = d.filter.recoverFilter(ctx)
	if err != nil {
		log.Errorf("filter.recoverFilter error %v", err)
	}

	err = d.tags.recoverRoomTag(ctx)
	if err != nil {
		log.Errorf("tags.recoverRoomTag error %v", err)
	}

	err = d.userInfo.recoverUserInfo(ctx)
	if err != nil {
		log.Errorf("userInfo.recoverUserInfo error %v", err)
	}

	log.Info("account db load finished")
}

func (d *Database) WriteDBEvent(ctx context.Context, update *dbtypes.DBEvent) error {
	span, _ := common.StartSpanFromContext(ctx, d.topic)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, d.topic, d.underlying)
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic,
		&core.TransportPubMsg{
			Keys:    []byte(update.GetEventKey()),
			Obj:     update,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}

func (d *Database) WriteDBEventWithTbl(ctx context.Context, update *dbtypes.DBEvent, tbl string) error {
	span, _ := common.StartSpanFromContext(ctx, d.topic+"_"+tbl)
	defer span.Finish()
	common.ExportMetricsBeforeSending(span, d.topic+"_"+tbl, d.underlying)
	return common.GetTransportMultiplexer().SendWithRetry(
		d.underlying,
		d.topic+"_"+tbl,
		&core.TransportPubMsg{
			Keys:    []byte(update.GetEventKey()),
			Obj:     update,
			Headers: common.InjectSpanToHeaderForSending(span),
		})
}

// CreateAccount makes a new account with the given login name and password, and creates an empty profile
// for this account. If no password is supplied, the account will be a passwordless account. If the
// account already exists, it will return nil, nil.
func (d *Database) CreateAccount(
	ctx context.Context, userID, plaintextPassword, appServiceID, displayName string,
) (*authtypes.Account, error) {
	// Generate a password hash if this is not a password-less user
	hash := ""
	if plaintextPassword != "" {
		hash, _ = hashPassword(plaintextPassword)
	}
	if err := d.profiles.initProfile(ctx, userID, displayName, ""); err != nil {
		return nil, err
	}
	if err := d.userInfo.initUserInfo(ctx, userID, displayName, "", "", "", "", 0); err != nil {
		return nil, err
	}
	return d.accounts.insertAccount(ctx, userID, hash, appServiceID)
}

func (d *Database) CreateAccountWithCheck(
	ctx context.Context, oldAccount *authtypes.Account, userID, plaintextPassword, appServiceID, displayName string,
) (*authtypes.Account, error) {
	// Generate a password hash if this is not a password-less user
	hash := ""
	if plaintextPassword != "" {
		hash, _ = hashPassword(plaintextPassword)
	}
	if err := d.profiles.initProfile(ctx, userID, displayName, ""); err != nil {
		return nil, err
	}
	if err := d.userInfo.initUserInfo(ctx, userID, displayName, "", "", "", "", 0); err != nil {
		return nil, err
	}
	if oldAccount == nil || oldAccount.UserID == "" {
		return d.accounts.insertAccount(ctx, userID, hash, appServiceID)
	} else {
		if oldAccount.AppServiceID != "actual" && appServiceID == "actual" {
			oldAccount.AppServiceID = appServiceID
			if err := d.accounts.updateAccount(ctx, userID, appServiceID); err != nil {
				return oldAccount, err
			}
		}
		return oldAccount, nil
	}
}

func (d *Database) GetAccount(ctx context.Context, userID string) (*authtypes.Account, error) {
	return d.accounts.selectAccount(ctx, userID)
}

func (d *Database) UpsertProfile(ctx context.Context, userID, displayName, avatarURL string,
) error {
	return d.profiles.upsertProfile(ctx, userID, displayName, avatarURL)
}
func (d *Database) UpsertProfileSync(ctx context.Context, userID, displayName, avatarURL string,
) error {
	return d.profiles.upsertProfileSync(ctx, userID, displayName, avatarURL)
}

func (d *Database) UpsertDisplayName(ctx context.Context, userID, displayName string,
) error {
	return d.profiles.upsertDisplayName(ctx, userID, displayName)
}
func (d *Database) UpsertDisplayNameSync(ctx context.Context, userID, displayName string,
) error {
	return d.profiles.upsertDisplayNameSync(ctx, userID, displayName)
}

func (d *Database) UpsertAvatar(ctx context.Context, userID, avatarURL string,
) error {
	return d.profiles.upsertAvatar(ctx, userID, avatarURL)
}
func (d *Database) UpsertAvatarSync(ctx context.Context, userID, avatarURL string,
) error {
	return d.profiles.upsertAvatarSync(ctx, userID, avatarURL)
}

func (d *Database) InsertAccount(
	ctx context.Context, userID, plaintextPassword, appServiceID, displayName string,
) (*authtypes.Account, error) {
	var err error
	hash := ""
	if plaintextPassword != "" {
		hash, err = hashPassword(plaintextPassword)
		if err != nil {
			return nil, err
		}
	}

	return d.accounts.insertAccount(ctx, userID, hash, appServiceID)
}

func hashPassword(plaintext string) (hash string, err error) {
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	return string(hashBytes), err
}

// PutFilter puts the passed filter into the database.
// Returns the filterID as a string. Otherwise returns an error if something
// goes wrong.
func (d *Database) PutFilter(
	ctx context.Context, userID, filterID, filter string,
) error {
	return d.filter.insertFilter(ctx, filter, filterID, userID)
}

func (d *Database) AddRoomTag(
	ctx context.Context, userId, roomID, tag string, content []byte,
) error {
	return d.tags.insertRoomTag(ctx, userId, roomID, tag, content)
}

func (d *Database) DeleteRoomTag(
	ctx context.Context, userId, roomID, tag string,
) error {
	return d.tags.deleteRoomTag(ctx, userId, roomID, tag)
}

func (d *Database) GetAccountsTotal(
	ctx context.Context,
) (int, error) {
	start := time.Now()

	res, err := d.accounts.selectAccountsTotal(ctx)

	duration := float64(time.Since(start)) / float64(time.Millisecond)
	d.qryDBGauge.WithLabelValues("GetAccountsTotal").Set(duration)

	return res, err
}

func (d *Database) GetActualTotal(
	ctx context.Context,
) (int, error) {
	return d.accounts.selectActualTotal(ctx)
}

func (d *Database) GetRoomTagsTotal(
	ctx context.Context,
) (int, error) {
	return d.tags.selectRoomTagsTotal(ctx)
}

// SaveAccountData saves new account data for a given user and a given room.
// If the account data is not specific to a room, the room ID should be an empty string
// If an account data already exists for a given set (user, room, data type), it will
// update the corresponding row with the new content
// Returns a SQL error if there was an issue with the insertion/update
func (d *Database) SaveAccountData(
	ctx context.Context, userID, roomID, dataType, content string,
) error {
	return d.accountData.insertAccountData(ctx, userID, roomID, dataType, content)
}

func (d *Database) GetAccountDataTotal(
	ctx context.Context,
) (int, error) {
	return d.accountData.selectAccountDataTotal(ctx)
}

func (d *Database) OnInsertAccountData(
	ctx context.Context, userID, roomID, dataType, content string,
) error {
	return d.accountData.onInsertAccountData(ctx, userID, roomID, dataType, content)
}

func (d *Database) OnInsertAccount(
	ctx context.Context, userID, hash, appServiceID string, createdTs int64,
) error {
	return d.accounts.onInsertAccount(ctx, userID, hash, appServiceID, createdTs)
}

func (d *Database) OnInsertFilter(
	ctx context.Context, filter, filterID, userID string,
) error {
	return d.filter.onInsertFilter(ctx, filter, filterID, userID)
}

func (d *Database) OnUpsertProfile(
	ctx context.Context, userID, displayName, avatarURL string,
) error {
	return d.profiles.onUpsertProfile(ctx, userID, displayName, avatarURL)
}

func (d *Database) OnInitProfile(
	ctx context.Context, userID, displayName, avatarURL string,
) error {
	return d.profiles.onInitProfile(ctx, userID, displayName, avatarURL)
}

func (d *Database) OnUpsertDisplayName(
	ctx context.Context, userID, displayName string,
) error {
	return d.profiles.onUpsertDisplayName(ctx, userID, displayName)
}

func (d *Database) OnUpsertAvatar(
	ctx context.Context, userID, avatarURL string,
) error {
	return d.profiles.onUpsertAvatar(ctx, userID, avatarURL)
}

func (d *Database) OnInsertRoomTag(
	ctx context.Context, userId, roomID, tag string, content []byte,
) error {
	return d.tags.onInsertRoomTag(ctx, userId, roomID, tag, content)
}

func (d *Database) OnDeleteRoomTag(
	ctx context.Context, userId, roomID, tag string,
) error {
	return d.tags.onDeleteRoomTag(ctx, userId, roomID, tag)
}

func (d *Database) GetAllProfile() ([]authtypes.Profile, error) {
	return d.profiles.getAllProfile()
}

func (d *Database) GetProfileByUserID(ctx context.Context, userID string) (authtypes.Profile, error) {
	return d.profiles.getProfile(ctx, userID)
}

func (d *Database) UpsertUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	return d.userInfo.upsertUserInfo(ctx, userID, userName, jobNumber, mobile, landline, email, state)
}

func (d *Database) OnUpsertUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	return d.userInfo.onUpsertUserInfo(ctx, userID, userName, jobNumber, mobile, landline, email, state)
}

func (d *Database) OnInitUserInfo(
	ctx context.Context, userID, userName, jobNumber, mobile, landline, email string, state int,
) error {
	return d.userInfo.onInitUserInfo(ctx, userID, userName, jobNumber, mobile, landline, email, state)
}

func (d *Database) GetAllUserInfo() ([]authtypes.UserInfo, error) {
	return d.userInfo.getAllUserInfo()
}

func (d *Database) DeleteUserInfo(
	ctx context.Context, userID string,
) error {
	return d.userInfo.deleteUserInfo(ctx, userID)
}

func (d *Database) OnDeleteUserInfo(
	ctx context.Context, userID string,
) error {
	return d.userInfo.onDeleteUserInfo(ctx, userID)
}

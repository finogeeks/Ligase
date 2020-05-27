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

package common

import (
	"context"

	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type RawCache interface {
	GetProfileByUserID(userID string) *authtypes.Profile
	GetProfileLessByUserID(userID string) (string, string, bool)
	GetAvatarURLByUser(userID string) (string, bool)
	GetDisplayNameByUser(userID string) (string, bool)

	SetProfile(userID, displayName, avatar string) error
	ExpireProfile(userID string) error

	DelProfile(userID string) error
	DelAvatar(userID string) error
	DelDisplayName(userID string) error
}

/*
type accountDB interface {
	UpsertProfileSync(ctx context.Context, userID, displayName, avatarURL string) error
	UpsertDisplayNameSync(ctx context.Context, userID, displayName string) error
	UpsertAvatarSync(ctx context.Context, userID, avatarURL string) error
}
*/

type ComplexCache struct {
	accountDB model.AccountsDatabase
	cache     RawCache

	defaultAvatar string
}

func NewComplexCache(db model.AccountsDatabase, cache RawCache) *ComplexCache {
	return &ComplexCache{
		accountDB:     db,
		cache:         cache,
		defaultAvatar: "",
	}
}

func (c *ComplexCache) SetDefaultAvatarURL(avatarURL string) {
	c.defaultAvatar = avatarURL
}

func (c *ComplexCache) SetProfile(
	ctx context.Context,
	userID string,
	displayName string,
	avatarURL string,
) error {
	err := c.accountDB.UpsertProfileSync(ctx, userID, displayName, avatarURL)
	if err != nil {
		log.Warnf("ComplexCache, upsert profile userID: %s, erro: %v", userID, err)
		return err
	}

	if err = c.cache.DelProfile(userID); err != nil {
		log.Warnf("ComplexCache, del profile userID: %s, erro: %v", userID, err)

		// retry through kafka
	}

	return nil
}

func (c *ComplexCache) SetAvatarURL(
	ctx context.Context,
	userID string,
	avatarURL string,
) error {
	err := c.accountDB.UpsertAvatarSync(ctx, userID, avatarURL)
	if err != nil {
		log.Warnf("ComplexCache, upsert avatarURL userID: %s, erro: %v", userID, err)
		return err
	}

	err = c.cache.DelAvatar(userID)
	if err != nil {
		log.Warnf("ComplexCache, del avatarURL userID: %s, erro: %v", userID, err)

		// retry through kafka

	}

	return nil
}

func (c *ComplexCache) SetDisplayName(
	ctx context.Context,
	userID string,
	displayName string,
) error {
	err := c.accountDB.UpsertDisplayNameSync(ctx, userID, displayName)
	if err != nil {
		log.Errorf("ComplexCache, upsert displayName userID: %s, erro: %v", userID, err)
		return err
	}

	err = c.cache.DelDisplayName(userID)
	if err != nil {
		log.Warnf("ComplexCache, del displayName userID: %s, erro: %v", userID, err)
		// keep retry through kafka

	}

	return nil
}

func (c *ComplexCache) GetProfileByUserID(
	ctx context.Context,
	userID string,
) (string, string, error) {
	displayName, avatarURL, ok := c.cache.GetProfileLessByUserID(userID)
	if ok {
		log.Infof("++++++++++++++++=ComplexCache, get profile from cache, userID: %s, displayName: %s, avatar: %s", userID, displayName, avatarURL)
		return displayName, avatarURL, nil
	}

	// trylock

	// get from DB
	profile, err := c.accountDB.GetProfileByUserID(ctx, userID)
	if err != nil {
		// set empty cache for nonexistent user
		profile.DisplayName = ""
		profile.AvatarURL = c.defaultAvatar
	}
	log.Warnf("++++++++++++++++=ComplexCache, get profile from DB, userID: %s, error: %v", userID, err)

	// TODO: use lua script to set value and expire
	// set cache
	err2 := c.cache.SetProfile(userID, profile.DisplayName, profile.AvatarURL)
	if err2 != nil {
		log.Warnf("ComplexCache, reset profile cache failed, err: %v", err2)
	}
	c.cache.ExpireProfile(userID)

	return profile.DisplayName, profile.AvatarURL, err
}

func (c *ComplexCache) GetAvatarURL(
	ctx context.Context,
	userID string,
) (string, error) {
	avatarURL, ok := c.cache.GetAvatarURLByUser(userID)
	if ok {
		log.Infof("++++++++++++++++=ComplexCache, get avatarURL from cache, userID: %s, avatar: %s", userID, avatarURL)
		return avatarURL, nil
	}

	// trylock

	// get from DB
	profile, err := c.accountDB.GetProfileByUserID(ctx, userID)
	if err != nil {
		// set empty cache for nonexistent user
		profile.DisplayName = ""
		profile.AvatarURL = c.defaultAvatar
	}
	log.Warnf("++++++++++++++++=ComplexCache, get avatar from DB, userID: %s, displayName: %s, avatar: %s, err: %v", userID, profile.DisplayName, profile.AvatarURL, err)

	// TODO: use lua script to set value and expire
	// set cache
	err2 := c.cache.SetProfile(userID, profile.DisplayName, profile.AvatarURL)
	if err2 != nil {
		log.Warnf("ComplexCache, reset profile cache failed, err: %v", err2)
	}
	c.cache.ExpireProfile(userID)

	return profile.AvatarURL, err
}

func (c *ComplexCache) GetDisplayName(
	ctx context.Context,
	userID string,
) (string, error) {
	displayName, ok := c.cache.GetDisplayNameByUser(userID)
	if ok {
		log.Infof("++++++++++++++++=get displayName from cache, userID: %s, displayName: %s", userID, displayName)
		return displayName, nil
	}

	// trylock

	// get from DB
	profile, err := c.accountDB.GetProfileByUserID(ctx, userID)
	if err != nil {
		// set empty cache for nonexistent user
		profile.DisplayName = ""
		profile.AvatarURL = c.defaultAvatar
	}
	log.Warnf("++++++++++++++++=ComplexCache, get displayName from DB, userID: %s, displayName: %s, avatar: %s, err: %v", userID, profile.DisplayName, profile.AvatarURL, err)

	// TODO: use lua script to set value and expire
	// set cache
	err2 := c.cache.SetProfile(userID, profile.DisplayName, profile.AvatarURL)
	if err2 != nil {
		log.Warnf("ComplexCache, reset profile cache failed, err: %v", err2)
	}
	c.cache.ExpireProfile(userID)

	return profile.DisplayName, err
}

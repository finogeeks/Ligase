package repos

import (
	"context"
	"fmt"
	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/model/pushapitypes"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
	"sync"
)

type PushDataRepo struct {
	persist model.PushAPIDatabase
	pusherMap *sync.Map     	//key: user, value: sync.map (key: appId___pushKey, value: pusher)
	pushRuleMap *sync.Map   	//key: user, value: sync.map (key: ruleId, value: rule)
	pushRuleEnableMap *sync.Map //key: user, value: sync.map (key: ruleId, value: ruleEnable)
	deLimiter string
	cfg *config.Dendrite
}

func NewPushDataRepo(persist model.PushAPIDatabase, cfg *config.Dendrite) *PushDataRepo {
	return &PushDataRepo{
		persist: persist,
		pusherMap: new(sync.Map),
		pushRuleMap: new(sync.Map),
		pushRuleEnableMap: new(sync.Map),
		deLimiter: "___",
		cfg: cfg,
	}
}

func (pd *PushDataRepo) isRelated(userID string) bool {
	return common.IsRelatedRequest(userID, pd.cfg.MultiInstance.Instance, pd.cfg.MultiInstance.Total, pd.cfg.MultiInstance.MultiWrite)
}

//load push data from db to mem
func (pd* PushDataRepo) LoadHistory(ctx context.Context){
	pd.loadPusher(ctx)
	pd.loadPushRule(ctx)
	pd.loadPushRuleEnable(ctx)
}

//pusher
//load pusher from db to mem
func (pd* PushDataRepo) loadPusher(ctx context.Context){
	pushers, err := pd.persist.LoadPusher(ctx)
	if err != nil {
		log.Panicf("load pusher err:%v", err)
	}
	count := 0
	for _, pusher := range pushers {
		if pd.isRelated(pusher.UserName) {
			pd.AddPusher(ctx, pusher, false)
			count++
		}
	}
	log.Infof("instance:%d load pusher len:%d", pd.cfg.MultiInstance.Instance, count)
}

func (pd *PushDataRepo) AddPusher(ctx context.Context, pusher pushapitypes.Pusher, writeDB bool) error {
	//usename is empty, data is invalid, compatibility old data
	if pusher.UserName == "" {
		log.Warnf("add pusher user is empty, pusher:%+v", pusher)
		return nil
	}
	dataStr, err := json.Marshal(pusher.Data)
	if err != nil {
		log.Errorf("AddPusher push data:%+v json.Marshal:%v", pusher, err)
		return err
	}
	pusher.Data = string(dataStr)
	if v, ok := pd.pusherMap.Load(pusher.UserName); ok {
		userPusher := v.(*sync.Map)
		userPusher.Store(fmt.Sprintf("%s%s%s", pusher.AppId, pd.deLimiter, pusher.PushKey), pusher)
	}else{
		userPusher := new(sync.Map)
		userPusher.Store(fmt.Sprintf("%s%s%s", pusher.AppId, pd.deLimiter, pusher.PushKey), pusher)
		pd.pusherMap.Store(pusher.UserName, userPusher)
	}
	if writeDB {
		return pd.persist.AddPusher(ctx, pusher.UserName, pusher.ProfileTag, pusher.Kind, pusher.AppId, pusher.AppDisplayName, pusher.DeviceDisplayName, pusher.PushKey, pusher.PushKeyTs, pusher.Lang, dataStr, pusher.DeviceID)
	}else{
		return nil
	}
}

func (pd *PushDataRepo) GetPusher(ctx context.Context, userID string) ([]pushapitypes.Pusher, error){
	pushers := []pushapitypes.Pusher{}
	if v, ok := pd.pusherMap.Load(userID); ok {
		userPusher := v.(*sync.Map)
		userPusher.Range(func(key, value interface{}) bool {
			pusher := value.(pushapitypes.Pusher)
			pushers = append(pushers, pusher)
			return true
		})
	}else{
		return nil, nil
	}
	return pushers, nil
}

func (pd *PushDataRepo) GetPusherByDeviceID(ctx context.Context, userID, deviceID string) ([]pushapitypes.Pusher, error){
	pushers := []pushapitypes.Pusher{}
	if v, ok := pd.pusherMap.Load(userID); ok {
		userPusher := v.(*sync.Map)
		userPusher.Range(func(key, value interface{}) bool {
			pusher := value.(pushapitypes.Pusher)
			if pusher.DeviceID == deviceID {
				pushers = append(pushers, pusher)
			}
			return true
		})
	}else{
		return nil, nil
	}
	return pushers, nil
}

func (pd *PushDataRepo) DeleteUserPusher(ctx context.Context, userID, appID, pushKey string) (error) {
	if v, ok := pd.pusherMap.Load(userID); ok {
		userPusher := v.(*sync.Map)
		userPusher.Delete(fmt.Sprintf("%s%s%s", appID, pd.deLimiter, pushKey))
		return pd.persist.DeleteUserPushers(ctx, userID, appID, pushKey)
	}else{
		log.Warnf("DeleteUserPusher userID:%s appID:%s pushKey:%s mem not exsit", userID, appID, pushKey)
		return pd.persist.DeleteUserPushers(ctx, userID, appID, pushKey)
	}
}

func (pd *PushDataRepo) DeletePushersByKey(ctx context.Context, appID, pushKey string) error {
	pd.pusherMap.Range(func(key, value interface{}) bool {
		pushers := value.(*sync.Map)
		pushers.Range(func(k,v interface{}) bool {
			if k.(string) == fmt.Sprintf("%s%s%s", appID, pd.deLimiter, pushKey){
				pushers.Delete(k)
			}
			return true
		})
		return true
	})
	return pd.persist.DeletePushersByKey(ctx, appID, pushKey)
}

//pushrule
func (pd *PushDataRepo) loadPushRule(ctx context.Context){
	rules, err := pd.persist.LoadPushRule(ctx)
	if err != nil {
		log.Panicf("load push rule err:%v", err)
	}
	count := 0
	for _, rule := range rules {
		if pd.isRelated(rule.UserName) {
			pd.AddPushRule(ctx, rule, false)
			count++
		}
	}
	log.Infof("instance:%d load push rule len:%d", pd.cfg.MultiInstance.Instance, count)
}

func (pd *PushDataRepo) AddPushRule(ctx context.Context, pushRule pushapitypes.PushRuleData, writeDB bool) error {
	//usename is empty, data is invalid, compatibility old data
	if pushRule.UserName == "" {
		log.Warnf("add pushRule user is empty, pushRule:%+v", pushRule)
		return nil
	}
	if v, ok := pd.pushRuleMap.Load(pushRule.UserName); ok {
		userPushRule := v.(*sync.Map)
		userPushRule.Store(pushRule.RuleId, pushRule)
	}else{
		userPushRule := new(sync.Map)
		userPushRule.Store(pushRule.RuleId, pushRule)
		pd.pushRuleMap.Store(pushRule.UserName, userPushRule)
	}
	if writeDB {
		return pd.persist.AddPushRule(ctx, pushRule.UserName, pushRule.RuleId, pushRule.PriorityClass, pushRule.Priority, pushRule.Conditions, pushRule.Actions)
	}else{
		return nil
	}
}

func (pd *PushDataRepo) GetPushRule(ctx context.Context, userID string) ([]pushapitypes.PushRuleData, error){
	rules := []pushapitypes.PushRuleData{}
	if v, ok := pd.pushRuleMap.Load(userID); ok {
		userPushRule := v.(*sync.Map)
		userPushRule.Range(func(key, value interface{}) bool {
			pushRule := value.(pushapitypes.PushRuleData)
			rules = append(rules, pushRule)
			return true
		})
	}else{
		return nil, nil
	}
	return rules, nil
}

func (pd *PushDataRepo) GetPushRuleByID(ctx context.Context, userID, ruleID string)(*pushapitypes.PushRuleData, error){
	if v, ok := pd.pushRuleMap.Load(userID); ok {
		userPushRule := v.(*sync.Map)
		if val, exsit := userPushRule.Load(ruleID); exsit {
			rule := val.(pushapitypes.PushRuleData)
			return &rule, nil
		}else{
			return nil, nil
		}
	}else{
		return nil, nil
	}
}

func (pd *PushDataRepo) DeletePushRule(ctx context.Context, userID, ruleID string) error {
	if v, ok := pd.pushRuleMap.Load(userID); ok {
		userPushRule := v.(*sync.Map)
		userPushRule.Delete(ruleID)
		return pd.persist.DeletePushRule(ctx, userID, ruleID)
	}else{
		log.Warnf("DeletePushRule userID:%s ruleID:%s mem not exsit", userID, ruleID)
		return pd.persist.DeletePushRule(ctx, userID, ruleID)
	}
}

//pushruleenable
func (pd *PushDataRepo) loadPushRuleEnable(ctx context.Context){
	rulesEnable, err := pd.persist.LoadPushRuleEnable(ctx)
	if err != nil {
		log.Panicf("load push rule enable err:%v", err)
	}
	count := 0
	for _, ruleEnable := range rulesEnable {
		if pd.isRelated(ruleEnable.UserID) {
			pd.AddPushRuleEnable(ctx, ruleEnable, false)
			count++
		}
	}
	log.Infof("instance:%d load push rule enable len:%d", pd.cfg.MultiInstance.Instance, count)
}

func (pd *PushDataRepo) AddPushRuleEnable(ctx context.Context, pushRuleEnable pushapitypes.PushRuleEnable, writeDB bool) error {
	//userId is empty, data is invalid, compatibility old data
	if pushRuleEnable.UserID == "" {
		log.Warnf("add pushRuleEnable user is empty, pushRuleEnable:%+v", pushRuleEnable)
		return nil
	}
	if v, ok := pd.pushRuleEnableMap.Load(pushRuleEnable.UserID); ok {
		userPushRuleEnable := v.(*sync.Map)
		userPushRuleEnable.Store(pushRuleEnable.RuleID, pushRuleEnable)
	}else{
		userPushRuleEnable := new(sync.Map)
		userPushRuleEnable.Store(pushRuleEnable.RuleID, pushRuleEnable)
		pd.pushRuleEnableMap.Store(pushRuleEnable.UserID, userPushRuleEnable)
	}
	if writeDB {
		return pd.persist.AddPushRuleEnable(ctx, pushRuleEnable.UserID, pushRuleEnable.RuleID, pushRuleEnable.Enabled)
	}else{
		return nil
	}
}

func (pd *PushDataRepo) GetPushRuleEnables(ctx context.Context, userID string) ([]pushapitypes.PushRuleEnable, error){
	rulesEnable := []pushapitypes.PushRuleEnable{}
	if v, ok := pd.pushRuleEnableMap.Load(userID); ok {
		userPushRuleEnable := v.(*sync.Map)
		userPushRuleEnable.Range(func(key, value interface{}) bool {
			pushRuleEnable := value.(pushapitypes.PushRuleEnable)
			rulesEnable = append(rulesEnable, pushRuleEnable)
			return true
		})
	}else{
		return nil, nil
	}
	return rulesEnable, nil
}

func (pd *PushDataRepo) GetPushRuleEnableByID(ctx context.Context, userID, ruleID string) (enabled, exsit bool) {
	if v, ok := pd.pushRuleEnableMap.Load(userID); ok {
		userPushRuleEnable := v.(*sync.Map)
		if val, e := userPushRuleEnable.Load(ruleID); e {
			pushRuleEnable := val.(pushapitypes.PushRuleEnable)
			return pushRuleEnable.Enabled == 1, true
		}else{
			return false, false
		}
	}else{
		return false, false
	}
}


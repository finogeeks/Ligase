package dbupdates

import (
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/dbupdates/consumers"
	_ "github.com/finogeeks/ligase/dbupdates/processors"
	"github.com/finogeeks/ligase/skunkworks/log"
)

func SetupDBUpdateComponent(cfg *config.Dendrite) {
	manager := consumers.NewDBEventSeqManager(cfg)
	if err := manager.Start(); err != nil {
		log.Errorf("DBEventSeqManager Start err %v", err)
	}
}

func SetupCacheUpdateComponent(cfg *config.Dendrite) {
	manager := consumers.NewCacheUpdateManager(cfg)
	if err := manager.Start(); err != nil {
		log.Errorf("CacheUpdateManager Start err %v", err)
	}
}

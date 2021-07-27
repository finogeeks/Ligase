package init

import (
	_ "github.com/finogeeks/ligase/dbupdates/processors/account"
	_ "github.com/finogeeks/ligase/dbupdates/processors/device"
	_ "github.com/finogeeks/ligase/dbupdates/processors/encryptapi"
	_ "github.com/finogeeks/ligase/dbupdates/processors/presence"
	_ "github.com/finogeeks/ligase/dbupdates/processors/publicroomsapi"
	_ "github.com/finogeeks/ligase/dbupdates/processors/pushapi"
	_ "github.com/finogeeks/ligase/dbupdates/processors/roomserver"
	_ "github.com/finogeeks/ligase/dbupdates/processors/syncapi"
)

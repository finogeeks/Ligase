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

package config

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/finogeeks/ligase/adapter"
	"github.com/finogeeks/ligase/model/authtypes"
	"github.com/finogeeks/ligase/plugins/message/external"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	jaegerconfig "github.com/uber/jaeger-client-go/config"
	jaegermetrics "github.com/uber/jaeger-lib/metrics"
	"golang.org/x/crypto/ed25519"
	"gopkg.in/yaml.v2"
)

// Version is the current version of the config format.
// This will change whenever we make breaking changes to the config format.
const Version = 0

const DebugRequest = false

const DefaultCompressLength = 1024 * 1024

var config *Dendrite

// Dendrite contains all the config used by a dendrite process.
// Relative paths are resolved relative to the current working directory
type Dendrite struct {
	// The version of the configuration file.
	// If the version in a file doesn't match the current dendrite config
	// version then we can give a clear error message telling the user
	// to update their config file to the current version.
	// The version of the file should only be different if there has
	// been a breaking change to the config file format.
	Version int `yaml:"version"`

	// The configuration required for a matrix server.
	Matrix struct {
		// The name of the server. This is usually the domain name, e.g 'matrix.org', 'localhost'.
		ServerName        []string `yaml:"server_name"`
		HomeServerURL     string   `yaml:"homeserver_url"`
		IdentityServerURL string   `yaml:"identity_server_url"`
		InstanceId        int      `yaml:"instance_id"`
		// Path to the private key which will be used to sign requests and events.
		PrivateKeyPath Path `yaml:"private_key"`
		// The private key which will be used to sign requests and events.
		PrivateKey ed25519.PrivateKey `yaml:"-"`
		// An arbitrary string used to uniquely identify the PrivateKey. Must start with the
		// prefix "ed25519:".
		KeyID gomatrixserverlib.KeyID `yaml:"-"`
		// List of paths to X509 certificates used by the external federation listeners.
		// These are used to calculate the TLS fingerprints to publish for this server.
		// Other matrix servers talking to this server will expect the x509 certificate
		// to match one of these certificates.
		// The certificates should be in PEM format.
		FederationCertificatePaths []Path `yaml:"federation_certificates"`
		// A list of SHA256 TLS fingerprints for the X509 certificates used by the
		// federation listener for this server.
		TLSFingerPrints []gomatrixserverlib.TLSFingerprint `yaml:"-"`
		// How long a remote server can cache our server key for before requesting it again.
		// Increasing this number will reduce the number of requests made by remote servers
		// for our key, but increases the period a compromised key will be considered valid
		// by remote servers.
		// Defaults to 24 hours.
		KeyValidityPeriod time.Duration `yaml:"key_validity_period"`
		// List of domains that the server will trust as identity servers to
		// verify third-party identifiers.
		// Defaults to an empty array.
		TrustedIDServers []string `yaml:"trusted_third_party_id_servers"`
		// If set, allows registration by anyone who also has the shared
		// secret, even if registration is otherwise disabled.
		RegistrationSharedSecret string `yaml:"registration_shared_secret"`
		// This Home Server's ReCAPTCHA public key.
		RecaptchaPublicKey string `yaml:"recaptcha_public_key"`
		// This Home Server's ReCAPTCHA private key.
		RecaptchaPrivateKey string `yaml:"recaptcha_private_key"`
		// Boolean stating whether catpcha registration is enabled
		// and required
		RecaptchaEnabled bool `yaml:"enable_registration_captcha"`
		// Secret used to bypass the captcha registration entirely
		RecaptchaBypassSecret string `yaml:"captcha_bypass_secret"`
		// HTTP API endpoint used to verify whether the captcha response
		// was successful
		RecaptchaSiteVerifyAPI string `yaml:"recaptcha_siteverify_api"`
		// If set disables new users from registering (except via shared
		// secrets)
		RegistrationDisabled bool `yaml:"registration_disabled"`
		ServerFromDB         bool `yaml:"server_from_db"`
	} `yaml:"matrix"`

	// The configuration specific to the media repostitory.
	Media struct {
		NetdiskUrl   string `yaml:"netdisk_url"`
		UploadUrl    string `yaml:"upload_url"`
		DownloadUrl  string `yaml:"download_url"`
		ThumbnailUrl string `yaml:"thumbnail_url"`
		MediaInfoUrl string `yaml:"mediainfo_url"`
	} `yaml:"media"`

	TransportConfs []TransportConf `yaml:"transport_configs"`

	ChannelConfs []ChannelConf `yaml:"channel_configs"`

	// The configuration for talking to kafka.
	Kafka struct {
		Statistics struct {
			ProducerInterval int `yaml:"producer_interval"`
			ConsumerInterval int `yaml:"consumer_interval"`
		} `yaml:"statistics"`
		CommonCfg struct {
			EnableIdempotence bool `yaml:"enable_idempotence"`
			ForceAsyncSend    bool `yaml:"force_async_send"`
			ReplicaFactor     int  `yaml:"replica_factor"`
			NumPartitions     int  `yaml:"num_partitions"`
			NumProducers      int  `yaml:"num_producers"`
		} `yaml:"common_cfg"`
		Producer struct {
			OutputRoomEvent    ProducerConf `yaml:"output_room_event"`
			InputRoomEvent     ProducerConf `yaml:"input_room_event"`
			OutputClientData   ProducerConf `yaml:"output_client_data"`
			OutputProfileData  ProducerConf `yaml:"output_profile_data"`
			DBUpdates          ProducerConf `yaml:"db_updates"`
			GetVisibilityRange ProducerConf `yaml:"get_visibility_range"`
			OutputRoomFedEvent ProducerConf `yaml:"output_room_fed_event"`
			FedEduUpdate       ProducerConf `yaml:"fed_edu_update"`
			EventRecover       ProducerConf `yaml:"output_room_event_recover"`
			FedBridgeOut       ProducerConf `yaml:"fed_bridge_out"`
			FedBridgeOutHs     ProducerConf `yaml:"fed_bridge_out_hs"`
			FedBridgeOutRes    ProducerConf `yaml:"fed_bridge_out_res"`
			DeviceStateUpdate  ProducerConf `yaml:"output_device_state_update"`
			SettingUpdate      ProducerConf `yaml:"setting_update"`
			UserInfoUpdate     ProducerConf `yaml:"user_info_update"`
			DismissRoom        ProducerConf `yaml:"dismiss_room"`
			OutputStatic       ProducerConf `yaml:"output_static_data"`
			DispatchOutput     ProducerConf `yaml:"dispatch_output"`
			FedAPIOutput       ProducerConf `yaml:"fedapi_output"`
			GetMissingEvent    ProducerConf `yaml:"get_missing_event"`
			DownloadMedia      ProducerConf `yaml:"download_media"`
		} `yaml:"producers"`
		Consumer struct {
			OutputRoomEventPublicRooms   ConsumerConf `yaml:"output_room_event_publicroom"`    // OutputRoomEventPublicRooms "public-rooms",
			OutputRoomEventAppservice    ConsumerConf `yaml:"output_room_event_appservice"`    // OutputRoomEventAppservice applicationService
			OutputRoomEventSyncServer    ConsumerConf `yaml:"output_room_event_syncserver"`    // OutputRoomEventSyncServer, "sync-server"
			OutputRoomEventSyncWriter    ConsumerConf `yaml:"output_room_event_syncwriter"`    // OutputRoomEventSyncServer, "sync-writer"
			OutputRoomEventSyncAggregate ConsumerConf `yaml:"output_room_event_syncaggregate"` // OutputRoomEventSyncServer, "sync-aggregate"

			InputRoomEvent             ConsumerConf `yaml:"input_room_event"`             // InputRoomEvent "roomserver"
			OutputClientData           ConsumerConf `yaml:"output_client_data"`           // OutputClientData "sync-api"
			OutputProfileSyncAggregate ConsumerConf `yaml:"output_profile_syncaggregate"` // OutputClientData "sync-api"
			OutputProfileSyncServer    ConsumerConf `yaml:"output_profile_syncserver"`    // OutputClientData "sync-api"
			CacheUpdates               ConsumerConf `yaml:"cache_updates"`                // DBUpdates persist-cache
			DBUpdates                  ConsumerConf `yaml:"db_updates"`                   // DBUpdates persist-db
			FedBridgeOut               ConsumerConf `yaml:"fed_bridge_out"`
			FedBridgeOutHs             ConsumerConf `yaml:"fed_bridge_out_hs"`
			FedBridgeOutRes            ConsumerConf `yaml:"fed_bridge_out_res"` // fed api in
			SettingUpdateSyncServer    ConsumerConf `yaml:"setting_update_syncserver"`
			SettingUpdateSyncAggregate ConsumerConf `yaml:"setting_update_syncaggregate"`
			SetttngUpdateProxy         ConsumerConf `yaml:"setting_update_proxy"`
			SettingUpdateContent       ConsumerConf `yaml:"setting_update_content"`
			SettingUpdateFed           ConsumerConf `yaml:"setting_update_fed"`
			DownloadMedia              ConsumerConf `yaml:"download_media"`
			DismissRoom                ConsumerConf `yaml:"dismiss_room"`

			DispatchInput   ConsumerConf `yaml:"dispatch_input"`
			SenderInput     ConsumerConf `yaml:"fedsenser_input"`
			FedAPIInput     ConsumerConf `yaml:"fedapi_input"`
			FedBackFill     ConsumerConf `yaml:"fed_backfill"`
			EduSenderInput  ConsumerConf `yaml:"edusender_input"`
			GetMissingEvent ConsumerConf `yaml:"get_missing_event"`
		} `yaml:"consumers"`
	} `yaml:"kafka"`

	Rpc struct {
		RsQryTopic                 string `yaml:"rs_qry_topic"`
		PrQryTopic                 string `yaml:"pr_qry_topic"`
		AliasTopic                 string `yaml:"alias_topic"`
		RoomInputTopic             string `yaml:"room_input_topic"`
		FedTopic                   string `yaml:"fed_topic"`
		FedAliasTopic              string `yaml:"fed_alias_topic"`
		FedProfileTopic            string `yaml:"fed_profile_topic"`
		FedAvatarTopic             string `yaml:"fed_avatar_topic"`
		FedDisplayNameTopic        string `yaml:"fed_displayname_topic"`
		FedRsQryTopic              string `yaml:"fed_rs_qry_topic"`
		FedDownloadTopic           string `yaml:"fed_download_topic"`
		FedInviteTopic             string `yaml:"fed_invite_topic"`
		FedUserInfoTopic           string `yaml:"fed_user_info_topic"`
		FedMakeJoinTopic           string `yaml:"fed_make_join_topic"`
		FedSendJoinTopic           string `yaml:"fed_send_join_topic"`
		FedMakeLeaveTopic          string `yaml:"fed_make_leave_topic"`
		FedSendLeaveTopic          string `yaml:"fed_send_leave_topic"`
		ProxyClientApiTopic        string `yaml:"proxy_client_api_topic"`
		ProxyEncryptoApiTopic      string `yaml:"proxy_encrypto_api_topic"`
		ProxyPublicRoomApiTopic    string `yaml:"proxy_publicroom_api_topic"`
		ProxyPushApiTopic          string `yaml:"proxy_push_api_topic"`
		ProxySyncApiTopic          string `yaml:"proxy_sync_api_topic"`
		ProxySyncWriterApiTopic    string `yaml:"proxy_sync_writer_api_topic"`
		ProxySyncAggregateApiTopic string `yaml:"proxy_syncaggregate_api_topic"`
		ProxyFedApiTopic           string `yaml:"proxy_fed_api_topic"`
		ProxyBgmgrApiTopic         string `yaml:"proxy_bgmgr_api_topic"`
		ProxyRCSServerApiTopic     string `yaml:"proxy_rcsserver_api_topic"`

		Driver        string  `yaml:"driver"`
		ConsulURL     string  `yaml:"consul_url"`
		SyncServer    RpcConf `yaml:"sync_server"`
		SyncAggregate RpcConf `yaml:"sync_aggregate"`
		Front         RpcConf `yaml:"front"`
		Proxy         RpcConf `yaml:"proxy"`
		Rcs           RpcConf `yaml:"rcs"`
		TokenWriter   RpcConf `yaml:"token_writer"`
		PublicRoom    RpcConf `yaml:"public_room"`
		RoomServer    RpcConf `yaml:"room_server"`
		Fed           RpcConf `yaml:"fed"`
		Push          RpcConf `yaml:"push"`

		FrontClientApiApi  RpcConf `yaml:"front_clientapi_api"`
		FrontBgMngApi      RpcConf `yaml:"front_bgmng_api"`
		FrontEncryptoApi   RpcConf `yaml:"front_encrypto_api"`
		FrontPublicRoomApi RpcConf `yaml:"front_publicroom_api"`
		FrontRcsApi        RpcConf `yaml:"front_rcs_api"`
		SyncServerPushApi  RpcConf `yaml:"sync_server_push_api"`
		SyncServerApi      RpcConf `yaml:"sync_server_api"`
		SyncAggregateApi   RpcConf `yaml:"sync_aggregate_api"`
		SyncWriterApi      RpcConf `yaml:"sync_writer_api"`
	} `yaml:"rpc"`

	Redis struct {
		Uris []string `yaml:"uris"`
	} `yaml:"redis"`
	Nats struct {
		Uri string `yaml:"uri"`
	} `yaml:"nats"`
	// Postgres Config
	Database struct {
		EnableBatch bool         `yaml:"enable_batch"`
		CreateDB    DataBaseConf `yaml:"create_db"`
		// The Account database stores the login details and account information
		// for local users. It is accessed by the ClientAPI.
		Account DataBaseConf `yaml:"account"`
		// The Device database stores session information for the devices of logged
		// in local users. It is accessed by the ClientAPI, the MediaAPI and the SyncAPI.
		Device DataBaseConf `yaml:"device"`
		// The ServerKey database caches the public keys of remote servers.
		// It may be accessed by the FederationAPI, the ClientAPI, and the MediaAPI.
		ServerKey DataBaseConf `yaml:"server_key"`
		// The SyncAPI stores information used by the SyncAPI server.
		// It is only accessed by the SyncAPI server.
		SyncAPI DataBaseConf `yaml:"sync_api"`
		// The RoomServer database stores information about matrix rooms.
		// It is only accessed by the RoomServer.
		RoomServer DataBaseConf `yaml:"room_server"`
		// The PublicRoomsAPI database stores information used to compute the public
		// room directory. It is only accessed by the PublicRoomsAPI server.
		PublicRoomsAPI DataBaseConf `yaml:"public_rooms_api"`
		// presence database
		Presence DataBaseConf `yaml:"presence"`
		// Encryption api database
		EncryptAPI DataBaseConf `yaml:"encrypt_api"`
		// The Naffka database is used internally by the naffka library, if used.
		Naffka DataBaseConf `yaml:"naffka,omitempty"`
		//Push server database
		PushAPI DataBaseConf `yaml:"push_api"`

		ApplicationService DataBaseConf `yaml:"application_service"`

		ServerConf DataBaseConf `yaml:"server_conf"`

		Content DataBaseConf `yaml:"content"`

		RCSServer DataBaseConf `yaml:"rcs_server"`

		Federation DataBaseConf `yaml:"federation"`

		UseSync bool `yaml:"use_sync"`
	} `yaml:"database"`

	// TURN Server Config
	TURN struct {
		// TODO Guest Support
		// Whether or not guests can request TURN credentials
		//AllowGuests bool `yaml:"turn_allow_guests"`
		// How long the authorization should last
		UserLifetime string `yaml:"turn_user_lifetime"`
		// The list of TURN URIs to pass to clients
		URIs []string `yaml:"turn_uris"`

		// Authorization via Shared Secret
		// The shared secret from coturn
		SharedSecret string `yaml:"turn_shared_secret"`

		// Authorization via Static Username & Password
		// Hardcoded Username and Password
		Username string `yaml:"turn_username"`
		Password string `yaml:"turn_password"`
	} `yaml:"turn"`

	// The internal addresses the components will listen on.
	// These should not be exposed externally as they expose metrics and debugging APIs.
	Listen struct {
		MediaAPI       Address `yaml:"media_api"`
		ClientAPI      Address `yaml:"client_api"`
		FederationAPI  Address `yaml:"federation_api"`
		SyncAPI        Address `yaml:"sync_api"`
		RoomServer     Address `yaml:"room_server"`
		PublicRoomsAPI Address `yaml:"public_rooms_api"`
		PushAPI        Address `yaml:"push_api"`
	} `yaml:"listen"`

	// The config for tracing the dendrite servers.
	Tracing struct {
		// The config for the jaeger opentracing reporter.
		Jaeger jaegerconfig.Configuration `yaml:"jaeger"`
	} `yaml:"tracing"`

	// Application Services
	// https://matrix.org/docs/spec/application_service/unstable.html
	ApplicationServices struct {
		// Configuration files for various application services
		ConfigFiles []string `yaml:"config_files"`
	} `yaml:"application_services"`

	Authorization struct {
		// Configuration for login authorize mode
		AuthorizeMode string `yaml:"login_authorize_mode"`
		AuthorizeCode string `yaml:"login_authorize_code"`
	} `yaml:"authorization"`

	PushService struct {
		// Configuration for push service
		RemoveFailTimes      int    `yaml:"remove_fail_times"`
		PushServerUrl        string `yaml:"push_server_url"`
		AndroidPushServerUrl string `yaml:"android_push_server_url"`
	} `yaml:"push_service"`

	Log struct {
		Signaled       bool
		Level          string   `yaml:"level"`
		Files          []string `yaml:"files"`
		Underlying     string   `yaml:"underlying"`
		WriteToStdout  bool     `yaml:"write_to_stdout"`
		RedirectStderr bool     `yaml:"redirect_stderr"`
		ZapConfig      struct {
			MaxSize        int    `yaml:"max_size"`
			MaxBackups     int    `yaml:"max_backups"`
			MaxAge         int    `yaml:"max_age"`
			LocalTime      bool   `yaml:"localtime"`
			Compress       bool   `yaml:"compress"`
			JsonFormat     bool   `yaml:"json_format"`
			BtEnabled      bool   `yaml:"bt_enabled"`
			BtLevel        string `yaml:"bt_level"`
			FieldSeparator string `yaml:"field_separator"`
		} `yaml:"zap_config"`
	} `yaml:"log"`

	// Any information derived from the configuration options for later use.
	Derived struct {
		Registration struct {
			// Flows is a slice of flows, which represent one possible way that the client can authenticate a request.
			// http://matrix.org/docs/spec/HEAD/client_server/r0.3.0.html#user-interactive-authentication-api
			// As long as the generated flows only rely on config file options,
			// we can generate them on startup and store them until needed
			Flows []external.AuthFlow `json:"flows"`

			// Params that need to be returned to the client during
			// registration in order to complete registration stages.
			Params map[string]interface{} `json:"params"`
		}

		// Application Services parsed from their config files
		// The paths of which were given above in the main config file
		ApplicationServices []ApplicationService

		// A meta-regex compiled from all exclusive Application Service
		// Regexes. When a user registers, we check that their username
		// does not match any exclusive Application Service namespaces
		ExclusiveApplicationServicesUsernameRegexp *regexp.Regexp

		// TODO: Exclusive alias, room regexp's
	} `yaml:"-"`

	Migration struct {
		// Configuration for migration
		DomainName          string   `yaml:"domain_name"`
		UpdateAvatar        bool     `yaml:"update_avatar"`
		ProcessDevice       bool     `yaml:"process_device"`
		AppendWhenRoomExist bool     `yaml:"append_when_room_exist"`
		SynapseDB           string   `yaml:"synapse_db"`
		GoRoomDB            string   `yaml:"go_room_db"`
		GoAccountDB         string   `yaml:"go_account_db"`
		SynapseUrl          string   `yaml:"synapse_url"`
		MigrationList       []string `yaml:"migration_list"`
		IgnoreRooms         []string `yaml:"ignore_rooms"`
		RoomList            []string `yaml:"room_list"`
	} `yaml:"migration"`

	Cache struct {
		DurationDefault int `yaml:"durationDefault"`
		DurationRefresh int `yaml:"durationRefresh"`
	} `yaml:"cache"`

	Macaroon struct {
		Key string `yaml:"macaroonKey"`
		Id  string `yaml:"macaroonId"`
		Loc string `yaml:"macaroonLoc"`
	} `yaml:"macaroon"`

	EventSkip struct {
		Items []SkipItem `yaml:"skip_item_list"`
	} `yaml:"eventskip"`

	CompressLength int64 `yaml:"compress_length"`

	Lru struct {
		MaxEntries int `yaml:"max_entries"`
		GcPerNum   int `yaml:"gc_per_num"`
	} `yaml:"lru"`

	FlushDelay int `yaml:"flush_delay"`

	SyncMngChanNum uint32 `yaml:"sync_mng_chan_num"`

	RecoverPath string `yaml:"recover_path"`

	SendMemberEvent bool `yaml:"send_member_event"`

	UseMessageFilter bool `yaml:"use_message_filter"`

	CalculateReadCount bool `yaml:"calculate_read_count"`

	RetryFlushDB bool `yaml:"retry_flush_db"`

	PubLoginInfo bool `yaml:"pub_login_info"`

	UseEncrypt bool `yaml:"use_encrypt"`

	DefaultAvatar string `yaml:"default_avatar"`

	DebugLevel string `yaml:"debug_level"`

	TokenRewrite struct {
		StaffDomain  string `yaml:"staff_domain"`
		RetailDomain string `yaml:"retail_domain"`
		StaffDB      string `yaml:"staff_db"`
		RetailDB     string `yaml:"retail_db"`
	} `yaml:"token_rewrite"`

	MultiInstance struct {
		Instance        uint32 `yaml:"instance"`
		Total           uint32 `yaml:"total"`
		MultiWrite      bool   `yaml:"multi_write"`
		SyncServerTotal uint32 `yaml:"sync_server_total"`
	} `yaml:"multi_instance"`

	DeviceMng struct {
		ScanUnActive int64 `yaml:"scan_unactive"`
		KickUnActive int64 `yaml:"kick_unactive"`
	} `yaml:"device_mng"`

	StateMgr struct {
		StateNotify  bool  `yaml:"state_notify"`
		StateOffline int64 `yaml:"state_offline"`
		StateReport  int64 `yaml:"state_report"`
	} `yaml:"state_mgr"`

	Encryption struct {
		Enable bool   `yaml:"enable"`
		Key    string `yaml:"key"`
		Mirror bool   `yaml:"mirror"`
	} `yaml:"encryption"`

	NotaryService struct {
		CliHttpsEnable bool   `yaml:"cli_https_enable"`
		SrvHttpsEnable bool   `yaml:"srv_https_enable"`
		RootCAUrl      string `yaml:"root_ca_url"`
		CRLUrl         string `yaml:"crl_url"`
		CertUrl        string `yaml:"cert_url"`
	} `yaml:"notary_service"`

	ExternalMigration struct {
		Enable   bool   `yaml:"enable"`
		Services string `yaml:"services"`
		MongoURL string `yaml:"mongo_url"`
	} `yaml:"external_migration"`

	License      string `yaml:"license"`
	LicenseItem  LicenseConf
	TokenExpire  int64  `yaml:"token_expire"`
	UtlExpire    int64  `yaml:"utl_expire"`
	LatestToken  int    `yaml:"latest_token"`
	ReceiptDelay int64  `yaml:"receipt_delay"`
	CheckReceipt int64  `yaml:"check_receipt"`
	OnlineSpec   string `yaml:"online_spec"`
	OnlineDetail string `yaml:"online_detail_spec"`
	GcTimer      int64  `yaml:"gc_timer"`

	Sync struct {
		RpcTimeout      int64 `yaml:"rpc_timeout"`
		FullSyncTimeout int64 `yaml:"fullsync_rpc_timeout"`
		Visibility      int64 `yaml:"visibility"`
	} `yaml:"sync"`
}

type LicenseConf struct {
	OrganName   string `json:"organ_name"`
	ExpireTime  int64  `json:"expire_time"`
	TotalUsers  int64  `json:"total_users"`
	TotalRooms  int64  `json:"total_rooms"`
	RoomMembers int64  `json:"room_members"`
	Desc        string `json:"desc"`
	OrganDomain string `json:"organ_domain"`
	EegOrgan    string `json:"reg_organ"`
	Encryption  bool   `json:"encryption"`
	Secret      string `json:"secret"`
}

type SkipItem struct {
	Patten string `yaml:"patten"`
	IsReg  bool   `yaml:"is_reg"`
}

type TransportConf struct {
	Addresses  string `yaml:"addresses"`
	Underlying string `yaml:"underlying"`
	Name       string `yaml:"name"`
}

type ChannelConf struct {
	TransportName string `yaml:"transport_name"`
	Name          string `yaml:"name"`
	Topic         string `yaml:"topic"`
	Group         string `yaml:"group"`
	Dir           string `yaml:"dir"`
}

type ConsumerConf struct {
	Topic      string `yaml:"topic"`
	Group      string `yaml:"group"`
	Underlying string `yaml:"underlying"`
	Name       string `yaml:"name"`

	AutoCommit       *bool   `yaml:"enable_auto_commit,omitempty"`
	CommitIntervalMS *int    `yaml:"auto_commit_interval_ms,omitempty"`
	AutoOffsetReset  *string `yaml:"topic_auto_offset_reset,omitempty"`
	EnableGoChannel  *bool   `yaml:"go_channel_enable,omitempty"`
}

func (c *ConsumerConf) EnableAutoCommit() *bool {
	return c.AutoCommit
}

func (c *ConsumerConf) AutoCommitIntervalMS() *int {
	return c.CommitIntervalMS
}

func (c *ConsumerConf) TopicAutoOffsetReset() *string {
	return c.AutoOffsetReset
}

func (c *ConsumerConf) GoChannelEnable() *bool {
	return c.EnableGoChannel
}

type ProducerConf struct {
	Topic      string `yaml:"topic"`
	Underlying string `yaml:"underlying"`
	Name       string `yaml:"name"`
	Inst       int    `yaml:"inst"` //producer instance number, default one instance
}

type RpcConf struct {
	Port            int      `yaml:"port"`
	ServerName      string   `yaml:"server_name"`
	ConsulTagPrefix string   `yaml:"consul_tag_prefix"`
	AddressesStr    string   `yaml:"addresses"`
	Addresses       []string `yaml:"-"`
}

type DataBaseConf struct {
	Driver    string `yaml:"driver"`
	Addresses string `yaml:"addresses"`
}

// A Path on the filesystem.
type Path string

// A DataSource for opening a postgresql database using lib/pq.
type DataSource string

// A Topic in kafka.
//type Topic string

// An Address to listen on.
type Address string

func GetConfig() *Dendrite {
	return config
}

func SetConfig(cfg *Dendrite) {
	config = cfg
}

// Load a yaml config file for a server run as multiple processes.
// Checks the config to ensure that it is valid.
// The checks are different if the server is run as a monolithic process instead
// of being split into multiple components
func Load(configPath string) error {
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	basePath, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	// Pass the current working directory and ioutil.ReadFile so that they can
	// be mocked in the tests
	monolithic := false
	return loadConfig(basePath, configData, ioutil.ReadFile, monolithic)
}

// LoadMonolithic loads a yaml config file for a server run as a single monolith.
// Checks the config to ensure that it is valid.
// The checks are different if the server is run as a monolithic process instead
// of being split into multiple components
func LoadMonolithic(configPath string) error {
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	basePath, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	// Pass the current working directory and ioutil.ReadFile so that they can
	// be mocked in the tests
	monolithic := true
	return loadConfig(basePath, configData, ioutil.ReadFile, monolithic)
}

// An Error indicates a problem parsing the config.
type Error struct {
	// List of problems encountered parsing the config.
	Problems []string
}

func loadConfig(
	basePath string,
	configData []byte,
	readFile func(string) ([]byte, error),
	monolithic bool,
) error {
	var err error
	config = new(Dendrite)
	if err = yaml.Unmarshal(configData, config); err != nil {
		return err
	}

	config.setDefaults()

	if err = config.check(monolithic); err != nil {
		return err
	}

	// Generate data from config options
	err = config.derive()
	if err != nil {
		return err
	}

	for _, val := range config.EventSkip.Items {
		gomatrixserverlib.AddSkipItem(val.Patten, val.IsReg)
	}

	if config.Authorization.AuthorizeCode == "" {
		b := make([]byte, 15)
		rand.Read(b)
		config.Authorization.AuthorizeCode = base64.RawURLEncoding.EncodeToString(b)
		fmt.Printf("loadConfig build default auth code:%s.", config.Authorization.AuthorizeCode)

	}
	adapter.SetKafkaEnableIdempotence(config.Kafka.CommonCfg.EnableIdempotence)
	adapter.SetKafkaForceAsyncSend(config.Kafka.CommonCfg.ForceAsyncSend)
	adapter.SetKafkaReplicaFactor(config.Kafka.CommonCfg.ReplicaFactor)
	adapter.SetKafkaNumPartitions(config.Kafka.CommonCfg.NumPartitions)
	adapter.SetKafkaNumProducers(config.Kafka.CommonCfg.NumProducers)
	adapter.SetDebugLevel(config.DebugLevel)
	adapter.SetCacheCfg(config.TokenExpire, config.UtlExpire, config.LatestToken)
	return nil
}

func (config *Dendrite) GetServerName() []string {
	return config.Matrix.ServerName
}

func (config *Dendrite) GetDBConfig(name string) (driver string, createAddr string, addr string, persistUnderlying string, persistName string, async bool) {
	switch name {
	case "accounts":
		return config.Database.Account.Driver, config.Database.CreateDB.Addresses, config.Database.Account.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "devices":
		return config.Database.Device.Driver, config.Database.CreateDB.Addresses, config.Database.Device.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "serverkey":
		return config.Database.ServerKey.Driver, config.Database.CreateDB.Addresses, config.Database.ServerKey.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "syncapi":
		return config.Database.SyncAPI.Driver, config.Database.CreateDB.Addresses, config.Database.SyncAPI.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "roomserver":
		return config.Database.RoomServer.Driver, config.Database.CreateDB.Addresses, config.Database.RoomServer.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "publicroomapi":
		return config.Database.PublicRoomsAPI.Driver, config.Database.CreateDB.Addresses, config.Database.PublicRoomsAPI.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "presence":
		return config.Database.Presence.Driver, config.Database.CreateDB.Addresses, config.Database.Presence.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "encryptoapi":
		return config.Database.EncryptAPI.Driver, config.Database.CreateDB.Addresses, config.Database.EncryptAPI.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "appservice":
		return config.Database.ApplicationService.Driver, config.Database.CreateDB.Addresses, config.Database.ApplicationService.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "pushapi":
		return config.Database.PushAPI.Driver, config.Database.CreateDB.Addresses, config.Database.PushAPI.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "server_conf":
		return config.Database.ServerConf.Driver, config.Database.CreateDB.Addresses, config.Database.ServerConf.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "content":
		return config.Database.Content.Driver, config.Database.CreateDB.Addresses, config.Database.Content.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "rcsserver":
		return config.Database.Content.Driver, config.Database.CreateDB.Addresses, config.Database.RCSServer.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	case "federation":
		return config.Database.Federation.Driver, config.Database.CreateDB.Addresses, config.Database.Federation.Addresses, config.Kafka.Producer.DBUpdates.Underlying, config.Kafka.Producer.DBUpdates.Name, !config.Database.UseSync
	default:
		return "", "", "", "", "", false
	}
}

// derive generates data that is derived from various values provided in
// the config file.
func (config *Dendrite) derive() error {
	// Determine registrations flows based off config values

	config.Derived.Registration.Params = make(map[string]interface{})

	// TODO: Add email auth type
	// TODO: Add MSISDN auth type

	if config.Matrix.RecaptchaEnabled {
		config.Derived.Registration.Params[authtypes.LoginTypeRecaptcha] = map[string]string{"public_key": config.Matrix.RecaptchaPublicKey}
		config.Derived.Registration.Flows = append(config.Derived.Registration.Flows,
			external.AuthFlow{Stages: []string{authtypes.LoginTypeRecaptcha}})
	} else {
		config.Derived.Registration.Flows = append(config.Derived.Registration.Flows,
			external.AuthFlow{Stages: []string{authtypes.LoginTypeDummy}})
	}

	if config.Rpc.Driver == "grpc" {
		rpcConfs := []*RpcConf{
			&config.Rpc.SyncServer,
			&config.Rpc.SyncAggregate,
			&config.Rpc.Front,
			&config.Rpc.Proxy,
			&config.Rpc.Rcs,
			&config.Rpc.TokenWriter,
			&config.Rpc.PublicRoom,
			&config.Rpc.RoomServer,
			&config.Rpc.Fed,
			&config.Rpc.Push,
			&config.Rpc.FrontClientApiApi,
			&config.Rpc.FrontBgMngApi,
			&config.Rpc.FrontEncryptoApi,
			&config.Rpc.FrontPublicRoomApi,
			&config.Rpc.FrontRcsApi,
			&config.Rpc.SyncServerPushApi,
			&config.Rpc.SyncServerApi,
			&config.Rpc.SyncAggregateApi,
			&config.Rpc.SyncWriterApi,
		}
		// TODO: 这里在k8s中的hostname，替换地址
		// 后面看看如何优化
		hostname, hasHostName := os.LookupEnv("HOSTNAME")
		var hostnameSl []string
		if hasHostName {
			hostnameSl = strings.Split(hostname, "-")
		}
		for _, v := range rpcConfs {
			if v.AddressesStr == "" {
				return errors.New("yaml rpc.sync_server.addresses is empty in grpc mode")
			}
			if hasHostName {
				v.AddressesStr = strings.Replace(v.AddressesStr, hostnameSl[0]+":", "127.0.0.1:", -1)
			}
			v.Addresses = strings.Split(v.AddressesStr, ",")
		}
		if config.MultiInstance.SyncServerTotal != 0 {
			log.Warnf("yaml multi_instance.sync_server_total not 0 in grpc mode")
		}
		if config.MultiInstance.Total != 0 {
			log.Warnf("yaml multi_instance.total not 0 in grpc mode")
		}
		config.MultiInstance.SyncServerTotal = uint32(len(config.Rpc.SyncServer.Addresses))
		config.MultiInstance.Total = uint32(len(config.Rpc.SyncAggregate.Addresses))
	}
	if config.Rpc.Driver == "grpc_with_consul" && config.Rpc.ConsulURL == "" {
		return errors.New("yaml rpc.consul_url is empty in grpc_with_consul mode")
	}

	// Load application service configuration files
	if err := loadAppservices(config); err != nil {
		return err
	}

	return nil
}

// setDefaults sets default config values if they are not explicitly set.
func (config *Dendrite) setDefaults() {
	if config.Matrix.KeyValidityPeriod == 0 {
		config.Matrix.KeyValidityPeriod = 24 * time.Hour
	}

	if config.Matrix.TrustedIDServers == nil {
		config.Matrix.TrustedIDServers = []string{}
	}

	if config.DeviceMng.ScanUnActive == 0 {
		config.DeviceMng.ScanUnActive = 3600000 //1 hour
	}

	if config.DeviceMng.KickUnActive == 0 {
		config.DeviceMng.KickUnActive = 2592000000 //30 day
	}
}

// Error returns a string detailing how many errors were contained within an
// Error type.
func (e Error) Error() string {
	if len(e.Problems) == 1 {
		return e.Problems[0]
	}
	return fmt.Sprintf(
		"%s (and %d other problems)", e.Problems[0], len(e.Problems)-1,
	)
}

// check returns an error type containing all errors found within the config
// file.
func (config *Dendrite) check(monolithic bool) error {
	var problems []string

	if config.Version != Version {
		return Error{[]string{fmt.Sprintf(
			"unknown config version %q, expected %q", config.Version, Version,
		)}}
	}

	checkNotEmpty := func(key string, value string) {
		if value == "" {
			problems = append(problems, fmt.Sprintf("missing config key %q", key))
		}
	}

	checkNotZero := func(key string, value int64) {
		if value == 0 {
			problems = append(problems, fmt.Sprintf("missing config key %q", key))
		}
	}

	/*checkPositive := func(key string, value int64) {
		if value < 0 {
			problems = append(problems, fmt.Sprintf("invalid value for config key %q: %d", key, value))
		}
	}*/

	checkValidDuration := func(key, value string) {
		if _, err := time.ParseDuration(config.TURN.UserLifetime); err != nil {
			problems = append(problems, fmt.Sprintf("invalid duration for config key %q: %s", key, value))
		}
	}

	checkNotZero("matrix.server_name", int64(len(config.Matrix.ServerName)))
	//checkNotEmpty("matrix.private_key", string(config.Matrix.PrivateKeyPath))
	//checkNotZero("matrix.federation_certificates", int64(len(config.Matrix.FederationCertificatePaths)))

	if config.TURN.UserLifetime != "" {
		checkValidDuration("turn.turn_user_lifetime", config.TURN.UserLifetime)
	}

	checkNotEmpty("media.upload_url", string(config.Media.UploadUrl))
	checkNotEmpty("media.download_url", string(config.Media.DownloadUrl))
	checkNotEmpty("media.thumbnail_url", string(config.Media.ThumbnailUrl))

	if !monolithic {
		checkNotEmpty("listen.media_api", string(config.Listen.MediaAPI))
		checkNotEmpty("listen.client_api", string(config.Listen.ClientAPI))
		checkNotEmpty("listen.federation_api", string(config.Listen.FederationAPI))
		checkNotEmpty("listen.sync_api", string(config.Listen.SyncAPI))
		checkNotEmpty("listen.push_api", string(config.Listen.PushAPI))
		checkNotEmpty("listen.room_server", string(config.Listen.RoomServer))
	}

	if problems != nil {
		return Error{problems}
	}

	return nil
}

// absPath returns the absolute path for a given relative or absolute path.
func absPath(dir string, path Path) string {
	if filepath.IsAbs(string(path)) {
		// filepath.Join cleans the path so we should clean the absolute paths as well for consistency.
		return filepath.Clean(string(path))
	}
	return filepath.Join(dir, string(path))
}

func readKeyPEM(path string, data []byte) (gomatrixserverlib.KeyID, ed25519.PrivateKey, error) {
	for {
		var keyBlock *pem.Block
		keyBlock, data = pem.Decode(data)
		if data == nil {
			return "", nil, fmt.Errorf("no matrix private key PEM data in %q", path)
		}
		if keyBlock == nil {
			return "", nil, fmt.Errorf("keyBlock is nil %q", path)
		}
		if keyBlock.Type == "MATRIX PRIVATE KEY" {
			keyID := keyBlock.Headers["Key-ID"]
			if keyID == "" {
				return "", nil, fmt.Errorf("missing key ID in PEM data in %q", path)
			}
			if !strings.HasPrefix(keyID, "ed25519:") {
				return "", nil, fmt.Errorf("key ID %q doesn't start with \"ed25519:\" in %q", keyID, path)
			}
			_, privKey, err := ed25519.GenerateKey(bytes.NewReader(keyBlock.Bytes))
			if err != nil {
				return "", nil, err
			}
			return gomatrixserverlib.KeyID(keyID), privKey, nil
		}
	}
}

func fingerprintPEM(data []byte) *gomatrixserverlib.TLSFingerprint {
	for {
		var certDERBlock *pem.Block
		certDERBlock, data = pem.Decode(data)
		if data == nil {
			return nil
		}
		if certDERBlock.Type == "CERTIFICATE" {
			digest := sha256.Sum256(certDERBlock.Bytes)
			return &gomatrixserverlib.TLSFingerprint{SHA256: digest[:]}
		}
	}
}

// RoomServerURL returns an HTTP URL for where the roomserver is listening.
func (config *Dendrite) RoomServerURL() string {
	// Hard code the roomserver to talk HTTP for now.
	// If we support HTTPS we need to think of a practical way to do certificate validation.
	// People setting up servers shouldn't need to get a certificate valid for the public
	// internet for an internal API.
	return "http://" + string(config.Listen.RoomServer)
}

// SetupTracing configures the opentracing using the supplied configuration.
func (config *Dendrite) SetupTracing(serviceName string) (closer io.Closer, err error) {
	return config.Tracing.Jaeger.InitGlobalTracer(
		serviceName,
		jaegerconfig.Logger(Logger{}),
		jaegerconfig.Metrics(jaegermetrics.NullFactory),
	)
}

func (config *Dendrite) GetRpcConfig(service string) (*RpcConf, bool) {
	switch service {
	case config.Rpc.Front.ServerName:
		return &config.Rpc.Front, true
	case config.Rpc.SyncServer.ServerName:
		return &config.Rpc.SyncServer, true
	case config.Rpc.SyncAggregate.ServerName:
		return &config.Rpc.SyncAggregate, true
	case config.Rpc.Proxy.ServerName:
		return &config.Rpc.Proxy, true
	case config.Rpc.Rcs.ServerName:
		return &config.Rpc.Rcs, true
	case config.Rpc.TokenWriter.ServerName:
		return &config.Rpc.TokenWriter, true
	case config.Rpc.PublicRoom.ServerName:
		return &config.Rpc.PublicRoom, true
	case config.Rpc.RoomServer.ServerName:
		return &config.Rpc.RoomServer, true
	case config.Rpc.Fed.ServerName:
		return &config.Rpc.Fed, true
	case config.Rpc.Push.ServerName:
		return &config.Rpc.Push, true
	case config.Rpc.FrontClientApiApi.ServerName:
		return &config.Rpc.FrontClientApiApi, true
	case config.Rpc.FrontBgMngApi.ServerName:
		return &config.Rpc.FrontBgMngApi, true
	case config.Rpc.FrontEncryptoApi.ServerName:
		return &config.Rpc.FrontEncryptoApi, true
	case config.Rpc.FrontPublicRoomApi.ServerName:
		return &config.Rpc.FrontPublicRoomApi, true
	case config.Rpc.FrontRcsApi.ServerName:
		return &config.Rpc.FrontRcsApi, true
	case config.Rpc.SyncServerPushApi.ServerName:
		return &config.Rpc.SyncServerPushApi, true
	case config.Rpc.SyncServerApi.ServerName:
		return &config.Rpc.SyncServerApi, true
	case config.Rpc.SyncAggregateApi.ServerName:
		return &config.Rpc.SyncAggregateApi, true
	case config.Rpc.SyncWriterApi.ServerName:
		return &config.Rpc.SyncWriterApi, true
	}
	return nil, false
}

// Logger is a small wrapper that implements jaeger.Logger.
type Logger struct {
}

func (l Logger) Error(msg string) {
	log.Error(msg)
}

func (l Logger) Infof(msg string, args ...interface{}) {
	log.Infof(msg, args...)
}

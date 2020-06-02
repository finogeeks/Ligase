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

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/finogeeks/ligase/adapter"

	"github.com/finogeeks/ligase/skunkworks/log"
	"gopkg.in/yaml.v2"
)

var (
	fedConfig Fed
)

type stringAddress string

type Fed struct {
	Version    int `yaml:"version"`
	Homeserver struct {
		ServerName   []string `yaml:"server_name"`
		ServerFromDB bool     `yaml:"server_from_db"`
	} `yaml:"homeserver"`

	TransportConfs []TransportConf `yaml:"transport_configs"`
	// The configuration for talking to kafka.
	Kafka struct {
		CommonCfg struct {
			EnableIdempotence bool `yaml:"enable_idempotence"`
			ForceAsyncSend    bool `yaml:"force_async_send"`
			ReplicaFactor     int  `yaml:"replica_factor"`
			NumPartitions     int  `yaml:"num_partitions"`
			NumProducers      int  `yaml:"num_producers"`
		} `yaml:"common_cfg"`
		Producer struct {
			DBUpdates       ProducerConf `yaml:"db_updates"`
			DispatchOutput  ProducerConf `yaml:"dispatch_output"`
			FedAPIOutput    ProducerConf `yaml:"fedapi_output"`
			GetMissingEvent ProducerConf `yaml:"get_missing_event"`
			DownloadMedia   ProducerConf `yaml:"download_media"`
			InputRoomEvent  ProducerConf `yaml:"input_room_event"`
		} `yaml:"producers"`
		Consumer struct {
			DispatchInput   ConsumerConf `yaml:"dispatch_input"`
			SenderInput     ConsumerConf `yaml:"fedsenser_input"`
			FedAPIInput     ConsumerConf `yaml:"fedapi_input"`
			FedBackFill     ConsumerConf `yaml:"fed_backfill"`
			EduSenderInput  ConsumerConf `yaml:"edusender_input"`
			SettingUpdate   ConsumerConf `yaml:"setting_update"`
			GetMissingEvent ConsumerConf `yaml:"get_missing_event"`
		} `yaml:"consumers"`
	} `yaml:"kafka"`
	Nats struct {
		Uri string `yaml:"uri"`
	} `yaml:"nats"`
	Rpc struct {
		RsQryTopic          string `yaml:"rs_qry_topic"`
		PrQryTopic          string `yaml:"pr_qry_topic"`
		AliasTopic          string `yaml:"alias_topic"`
		RoomInputTopic      string `yaml:"room_input_topic"`
		FedTopic            string `yaml:"fed_topic"`
		FedAliasTopic       string `yaml:"fed_alias_topic"`
		FedProfileTopic     string `yaml:"fed_profile_topic"`
		FedAvatarTopic      string `yaml:"fed_avatar_topic"`
		FedDisplayNameTopic string `yaml:"fed_displayname_topic"`
		FedRsQryTopic       string `yaml:"fed_rs_qry_topic"`
		FedRsDownloadTopic  string `yaml:"fed_download_topic"`
		FedRsInviteTopic    string `yaml:"fed_invite_topic"`
		FedUserInfoTopic    string `yaml:"fed_user_info_topic"`
		FedRsMakeJoinTopic  string `yaml:"fed_makejoin_topic"`
		FedRsSendJoinTopic  string `yaml:"fed_sendjoin_topic"`
		FedRsMakeLeaveTopic string `yaml:"fed_makeleave_topic"`
		FedRsSendLeaveTopic string `yaml:"fed_sendleave_topic"`
	} `yaml:"rpc"`
	Media struct {
		UploadUrl string `yaml:"upload_url"`
	} `yaml:"media"`
	Database struct {
		CreateDB   DataBaseConf `yaml:"create_db"`
		RoomServer DataBaseConf `yaml:"room_server"`
		Federation DataBaseConf `yaml:"federation"`
		ServerConf DataBaseConf `yaml:"server_conf"`
		ServerKey  DataBaseConf `yaml:"server_key"`
		Encryption DataBaseConf `yaml:"encryption"`
		Account    DataBaseConf `yaml:"account"`
		UseSync    bool         `yaml:"use_sync"`
	} `yaml:"database"`
	Redis struct {
		Uris []string `yaml:"uris"`
	} `yaml:"redis"`
	Log struct {
		Debug          bool     `yaml:"debug"`
		Level          string   `yaml:"level"`
		Files          []string `yaml:"files"`
		Underlying     string   `yaml:"underlying"`
		WriteToStdout  bool     `yaml:"write_to_stdout"`
		RedirectStderr bool     `yaml:"redirect_stderr"`
		ZapConfig      struct {
			MaxSize    int    `yaml:"max_size"`
			MaxBackups int    `yaml:"max_backups"`
			MaxAge     int    `yaml:"max_age"`
			LocalTime  bool   `yaml:"localtime"`
			Compress   bool   `yaml:"compress"`
			JsonFormat bool   `yaml:"json_format"`
			BtEnabled  bool   `yaml:"bt_enabled"`
			BtLevel    string `yaml:"bt_level"`
		} `yaml:"zap_config"`
	} `yaml:"log"`

	Cache struct {
		DurationDefault int `yaml:"durationDefault"`
		DurationRefresh int `yaml:"durationRefresh"`
	} `yaml:"cache"`

	NotaryService struct {
		CliHttpsEnable bool   `yaml:"cli_https_enable"`
		RootCAUrl      string `yaml:"root_ca_url"`
		CRLUrl         string `yaml:"crl_url"`
		CertUrl        string `yaml:"cert_url"`
	} `yaml:"notary_service"`
}

type ConnectorConf struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type DataBaseConf struct {
	Driver    string `yaml:"driver"`
	Addresses string `yaml:"addresses"`
}

type TransportConf struct {
	Addresses  string `yaml:"addresses"`
	Underlying string `yaml:"underlying"`
	Name       string `yaml:"name"`
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
	Inst       int    `yaml:"inst"`

	LingerMs *string `yaml:"linger_ms,omitempty"`
}

func (p *ProducerConf) LingerMsConf() *string {
	return p.LingerMs
}

func GetFedConfig() Fed {
	return fedConfig
}

func (f Fed) GetServerName() []string {
	return f.Homeserver.ServerName
}

func (f Fed) GetServerFromDB() bool {
	return f.Homeserver.ServerFromDB
}

func (f Fed) GetMsgBusAddress() string {
	return f.Nats.Uri
}
func (f Fed) GetMsgBusReqTopic() string {
	return f.Nats.Uri
}
func (f Fed) GetMsgBusResTopic() string {
	return f.Nats.Uri
}

func (f Fed) GetDBConfig(name string) (driver string, createAddr string, addr string, persistUnderlying string, persistName string, async bool) {
	switch name {
	case "roomserver":
		return f.Database.RoomServer.Driver, f.Database.CreateDB.Addresses, f.Database.RoomServer.Addresses, f.Kafka.Producer.DBUpdates.Underlying, f.Kafka.Producer.DBUpdates.Name, !f.Database.UseSync
	case "federation":
		return f.Database.Federation.Driver, f.Database.CreateDB.Addresses, f.Database.Federation.Addresses, f.Kafka.Producer.DBUpdates.Underlying, f.Kafka.Producer.DBUpdates.Name, !f.Database.UseSync
	case "server_conf":
		return f.Database.ServerConf.Driver, f.Database.CreateDB.Addresses, f.Database.ServerConf.Addresses, f.Kafka.Producer.DBUpdates.Underlying, f.Kafka.Producer.DBUpdates.Name, !f.Database.UseSync
	case "serverkey":
		return f.Database.ServerKey.Driver, f.Database.CreateDB.Addresses, f.Database.ServerKey.Addresses, f.Kafka.Producer.DBUpdates.Underlying, f.Kafka.Producer.DBUpdates.Name, !f.Database.UseSync
	case "encryptoapi":
		return f.Database.Encryption.Driver, f.Database.CreateDB.Addresses, f.Database.Encryption.Addresses, f.Kafka.Producer.DBUpdates.Underlying, f.Kafka.Producer.DBUpdates.Name, !f.Database.UseSync
	case "accounts":
		return f.Database.Account.Driver, f.Database.CreateDB.Addresses, f.Database.Account.Addresses, f.Kafka.Producer.DBUpdates.Underlying, f.Kafka.Producer.DBUpdates.Name, !f.Database.UseSync
	default:
		return "", "", "", "", "", false
	}
}

func Load(configPath string) error {
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("read config file failed", "file:", configPath, "err: ", err)
		return err
	}

	return loadConfig(configData)
}

func loadConfig(configData []byte) error {
	if err := yaml.Unmarshal(configData, &fedConfig); err != nil {
		log.Fatal("parse config file failed", err)
		return err
	}
	adapter.SetKafkaEnableIdempotence(fedConfig.Kafka.CommonCfg.EnableIdempotence)
	adapter.SetKafkaForceAsyncSend(fedConfig.Kafka.CommonCfg.ForceAsyncSend)
	adapter.SetKafkaReplicaFactor(fedConfig.Kafka.CommonCfg.ReplicaFactor)
	adapter.SetKafkaNumPartitions(fedConfig.Kafka.CommonCfg.NumPartitions)
	adapter.SetKafkaNumProducers(fedConfig.Kafka.CommonCfg.NumProducers)
	logCfg := new(log.LogConfig)
	logCfg.Level = GetFedConfig().Log.Level
	logCfg.Files = GetFedConfig().Log.Files
	logCfg.WriteToStdout = GetFedConfig().Log.WriteToStdout
	logCfg.ZapConfig.MaxSize = GetFedConfig().Log.ZapConfig.MaxSize
	logCfg.ZapConfig.MaxBackups = GetFedConfig().Log.ZapConfig.MaxBackups
	logCfg.ZapConfig.MaxAge = GetFedConfig().Log.ZapConfig.MaxAge
	logCfg.ZapConfig.LocalTime = GetFedConfig().Log.ZapConfig.LocalTime
	logCfg.ZapConfig.Compress = GetFedConfig().Log.ZapConfig.Compress
	logCfg.ZapConfig.JsonFormat = GetFedConfig().Log.ZapConfig.JsonFormat
	logCfg.ZapConfig.BtEnabled = GetFedConfig().Log.ZapConfig.BtEnabled
	logCfg.ZapConfig.BtLevel = GetFedConfig().Log.ZapConfig.BtLevel
	log.Setup(logCfg)

	//add pid-file
	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		logDir, _ = filepath.Abs(filepath.Dir("."))
		logDir = logDir + "/log"
	}
	_ = os.Mkdir(logDir, os.ModePerm)
	f, _ := os.OpenFile(filepath.Join(logDir, "fed.pid"), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	// f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	f.WriteString(strconv.Itoa(os.Getpid())) // faster than fmt.Sprintf
	f.Close()
	return nil
}

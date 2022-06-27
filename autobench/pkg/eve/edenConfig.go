package evehelper

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/lf-edge/eden/pkg/defaults"
	"github.com/lf-edge/eden/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type EserverConfig struct {
	Port   int          `mapstructure:"port"`
	Force  bool         `mapstructure:"force"`
	Tag    string       `mapstructure:"tag"`
	Images ImagesConfig `mapstructure:"images"`
}

type ImagesConfig struct {
	EserverImageDist string `mapstructure:"dist"`
}

type EdenConfig struct {
	Download bool   `mapstructure:"download"`
	BinDir   string `mapstructure:"bin-dist"`
	CertsDir string `mapstructure:"certs-dist"`

	Eserver EserverConfig `mapstructure:"eserver"`

	Images ImagesConfig `mapstructure:"images"`
}

type RedisConfig struct {
	RemoteURL string `mapstructure:"adam"`
	Tag       string `mapstructure:"tag"`
	Port      int    `mapstructure:"port"`
	Dist      string `mapstructure:"dist"`
	Force     bool   `mapstructure:"force"`
}

type RemoteConfig struct {
	Redis bool `mapstructure:"redis"`
}

type AdamConfig struct {
	Tag         string `mapstructure:"tag"`
	Port        int    `mapstructure:"port"`
	Dist        string `mapstructure:"dist"`
	CertsDomain string `mapstructure:"domain"`
	CertsIP     string `mapstructure:"ip"`
	CertsEVEIP  string `mapstructure:"eve-ip"`
	ApiV1       bool   `mapstructure:"v1"`
	Force       bool   `mapstructure:"force"`

	Redis  RedisConfig  `mapstructure:"redis"`
	Remote RemoteConfig `mapstructure:"remote"`
}

type EveConfig struct {
	QemuFirmware         []string          `mapstructure:"firmware"`
	QemuConfigPath       string            `mapstructure:"config-part"`
	QemuDTBPath          string            `mapstructure:"dtb-part"`
	QemuOS               string            `mapstructure:"os"`
	GrubOptions          []string          `mapstructure:"grub-options"`
	ImageFile            string            `mapstructure:"image-file"`
	CertsUUID            string            `mapstructure:"uuid"`
	Dist                 string            `mapstructure:"dist"`
	Repo                 string            `mapstructure:"repo"`
	Registry             string            `mapstructure:"registry"`
	Tag                  string            `mapstructure:"tag"`
	UefiTag              string            `mapstructure:"uefi-tag"`
	HV                   string            `mapstructure:"hv"`
	Arch                 string            `mapstructure:"arch"`
	HostFwd              map[string]string `mapstructure:"hostfwd"`
	QemuFileToSave       string            `mapstructure:"qemu-config"`
	QemuCpus             int               `mapstructure:"cpu"`
	QemuMemory           int               `mapstructure:"ram"`
	ImageSizeMB          int               `mapstructure:"disk"`
	DevModel             string            `mapstructure:"devmodel"`
	Ssid                 string            `mapstructure:"ssid"`
	Serial               string            `mapstructure:"serial"`
	Accel                bool              `mapstructure:"accel"`
	QemuMonitorPort      int               `mapstructure:"eve.qemu-monitor-port"`
	QemuNetdevSocketPort int               `mapstructure:"eve.qemu-netdev-socket-port"`
	Pid                  string            `mapstructure:"pid"`
	Log                  string            `mapstructure:"log"`
	TelnetPort           int               `mapstructure:"eve.telnet-port"`
	Remote               bool              `mapstructure:"remote"`
	Swtpm                bool              `mapstructure:"eve.tpm"`
}

type RegistryConfig struct {
	Tag  string `mapstructure:"tag"`
	Port int    `mapstructure:"port"`
}

type EdenSetupArgs struct {
	EveConfigDir  string
	Netboot       bool
	Installer     bool
	Softserial    string
	ZedcontrolURL string
	ConfigName    string
	ConfigDir     string

	Eden     EdenConfig     `mapstructure:"eden"`
	Adam     AdamConfig     `mapstructure:"adam"`
	Eve      EveConfig      `mapstructure:"eve"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Registry RegistryConfig `mapstructure:"registry"`
}

func LoadConfig(configFile string) (*EdenSetupArgs, error) {
	viperLoaded, err := utils.LoadConfigFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %s", err.Error())
	}

	if !viperLoaded {
		return nil, fmt.Errorf("viper cannot be loaded")
	}
	viper.SetDefault("eve.uefi-tag", defaults.DefaultEVETag)

	cfg := &EdenSetupArgs{}

	if err = viper.Unmarshal(cfg); err != nil {
		log.Error("unable to decode into config struct, %v", err)
	}

	cfg.Eden.BinDir = utils.ResolveAbsPath(cfg.Eden.BinDir)
	cfg.Eden.CertsDir = utils.ResolveAbsPath(cfg.Eden.CertsDir)
	cfg.Eden.Images.EserverImageDist = utils.ResolveAbsPath(cfg.Eden.Images.EserverImageDist)

	cfg.Adam.Dist = utils.ResolveAbsPath(cfg.Adam.Dist)

	cfg.Eve.QemuDTBPath = utils.ResolveAbsPath(cfg.Eve.QemuDTBPath)
	cfg.Eve.ImageFile = utils.ResolveAbsPath(cfg.Eve.ImageFile)
	cfg.Eve.Dist = utils.ResolveAbsPath(cfg.Eve.Dist)
	cfg.Eve.QemuFileToSave = utils.ResolveAbsPath(cfg.Eve.QemuFileToSave)
	cfg.Eve.QemuConfigPath = utils.ResolveAbsPath(cfg.Eve.QemuConfigPath)
	cfg.Eve.Pid = utils.ResolveAbsPath(cfg.Eve.Pid)
	cfg.Eve.Log = utils.ResolveAbsPath(cfg.Eve.Log)

	cfg.Adam.Redis.Dist = utils.ResolveAbsPath(cfg.Adam.Redis.Dist)

	if configFile == "" {
		configFile, _ = utils.DefaultConfigPath()
	}

	cfg.ConfigName = path.Base(configFile)
	if pos := strings.LastIndexByte(cfg.ConfigName, '.'); pos != -1 {
		cfg.ConfigName = cfg.ConfigName[:pos]
	}

	configCheck(cfg.ConfigName)

	return cfg, nil
}

func configCheck(configName string) {
	configFile := utils.GetConfig(configName)
	configSaved := utils.ResolveAbsPath(fmt.Sprintf("%s-%s", configName, defaults.DefaultConfigSaved))

	abs, err := filepath.Abs(configSaved)
	if err != nil {
		log.Fatalf("fail in reading filepath: %s\n", err.Error())
	}

	if _, err = os.Lstat(abs); os.IsNotExist(err) {
		if err = utils.CopyFile(configFile, abs); err != nil {
			log.Fatalf("copying fail %s\n", err.Error())
		}
	} else {

		viperLoaded, err := utils.LoadConfigFile(abs)
		if err != nil {
			log.Fatalf("error reading config %s: %s\n", abs, err.Error())
		}
		if viperLoaded {
			confOld := viper.AllSettings()

			if _, err = utils.LoadConfigFile(configFile); err != nil {
				log.Fatalf("error reading config %s: %s", configFile, err.Error())
			}

			confCur := viper.AllSettings()

			if reflect.DeepEqual(confOld, confCur) {
				log.Infof("Config file %s is the same as %s\n", configFile, configSaved)
			} else {
				log.Fatalf("The current configuration file %s is different from the saved %s. You can fix this with the commands 'eden config clean' and 'eden config add/set/edit'.\n", configFile, abs)
			}
		} else {
			/* Incorrect saved config -- just rewrite by current */
			if err = utils.CopyFile(configFile, abs); err != nil {
				log.Fatalf("copying fail %s\n", err.Error())
			}
		}
	}
}

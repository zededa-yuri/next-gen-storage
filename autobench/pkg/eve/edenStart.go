package evehelper

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lf-edge/eden/pkg/defaults"
	"github.com/lf-edge/eden/pkg/eden"
	utils "github.com/lf-edge/eden/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func StartAdam(cfg EdenSetupArgs) error {
	command, err := os.Executable()
	if err != nil {
		return fmt.Errorf("startAdam: cannot obtain executable path: %s", err)
	}
	log.Infof("Executable path: %s", command)

	if !cfg.Adam.Remote.Redis {
		cfg.Adam.Redis.RemoteURL = ""
	}

	if err := eden.StartAdam(cfg.Adam.Port, cfg.Adam.Dist, cfg.Adam.Force, cfg.Adam.Tag, cfg.Adam.Redis.RemoteURL, cfg.Adam.ApiV1); err != nil {
		return fmt.Errorf("cannot start adam: %s", err)
	} else {
		log.Infof("Adam is runnig and accessible on port %d", cfg.Adam.Port)
	}
	return nil
}

func stopAdam(configFile string) error {
	viperLoaded, err := utils.LoadConfigFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading config : %s", err.Error())
	}
	if !viperLoaded {
		return fmt.Errorf("stopAdam: viper cannot be loaded")
	}

	adamRm := viper.GetBool("adam-rm")

	if err := eden.StopAdam(adamRm); err != nil {
		return fmt.Errorf("cannot stop adam: %s", err)
	}
	return nil
}

func GetAdamStatus() (string, error) {
	statusAdam, err := eden.StatusAdam()
	if err != nil {
		return "", fmt.Errorf("cannot obtain status of adam: %s", err)
	} else {
		return statusAdam, nil
	}
}

func StartRedis(cfg EdenSetupArgs) error {
	if err := eden.StartRedis(cfg.Redis.Port, cfg.Adam.Redis.Dist, cfg.Redis.Force, cfg.Redis.Tag); err != nil {
		return fmt.Errorf("cannot start redis: %s", err)
	} else {
		log.Infof("Redis is running and accessible on port %d", cfg.Redis.Port)
		return nil
	}
}

func StartRegistry(cfg EdenSetupArgs, registryDist string) error {

	if err := eden.StartRegistry(cfg.Registry.Port, cfg.Registry.Tag, registryDist); err != nil {
		return fmt.Errorf("cannot start registry: %s", err)
	} else {
		log.Infof("registry is running and accesible on port %d", cfg.Registry.Port)
	}
	return nil
}

func StartEServer(cfg EdenSetupArgs) error {
	if err := eden.StartEServer(cfg.Eden.Eserver.Port, cfg.Eden.Images.EserverImageDist, cfg.Eden.Eserver.Force, cfg.Eden.Eserver.Tag); err != nil {
		return fmt.Errorf("cannot start eserver: %s", err)
	} else {
		log.Infof("Eserver is running and accesible on port %d", cfg.Eden.Eserver.Port)
		return nil
	}
}

func StartEve(cfg EdenSetupArgs, vmName, tapInterface string) error {

	if cfg.Eve.DevModel == defaults.DefaultParallelsModel {
		if err := eden.StartEVEParallels(vmName, cfg.Eve.ImageFile, cfg.Eve.QemuCpus, cfg.Eve.QemuMemory, cfg.Eve.HostFwd); err != nil {
			return fmt.Errorf("cannot start eve: %s", err)
		} else {
			log.Infof("EVE is starting in Parallels")
		}
	} else if cfg.Eve.DevModel == defaults.DefaultVBoxModel {
		if err := eden.StartEVEVBox(vmName, cfg.Eve.ImageFile, cfg.Eve.QemuCpus, cfg.Eve.QemuMemory, cfg.Eve.HostFwd); err != nil {
			return fmt.Errorf("cannot start eve: %s", err)
		} else {
			log.Infof("EVE is starting in Virtual Box")
		}
	} else {
		if cfg.Eve.Swtpm {
			err := eden.StartSWTPM(filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "swtpm"))
			if err != nil {
				log.Errorf("cannot start swtpm: %s", err)
			} else {
				log.Infof("swtpm is starting")
			}
		}

		if err := eden.StartEVEQemu(cfg.Eve.Arch, cfg.Eve.QemuOS, cfg.Eve.ImageFile,
			cfg.Eve.Serial, cfg.Eve.TelnetPort, cfg.Eve.QemuMonitorPort,
			cfg.Eve.QemuNetdevSocketPort, cfg.Eve.HostFwd, cfg.Eve.Accel,
			cfg.Eve.QemuFileToSave, cfg.Eve.Log, cfg.Eve.Pid, tapInterface,
			0, cfg.Eve.Swtpm, false); err != nil {
			return fmt.Errorf("cannot start eve: %s", err)
		} else {
			log.Infof("EVE is starting")
		}
	}
	return nil
}

func StartEden(cfg EdenSetupArgs, registryDist, vmName, tapInterface string) error {

	if err := StartRedis(cfg); err != nil {
		return fmt.Errorf("cannot start adam %s", err)
	}

	if err := StartAdam(cfg); err != nil {
		return fmt.Errorf("cannot start adam %s", err)
	}

	if err := StartRegistry(cfg, registryDist); err != nil {
		return fmt.Errorf("cannot start registry %s", err)
	} else {
		log.Info("Registry is running and accesible on port %d", cfg.Registry.Port)
	}

	if cfg.Eve.Remote {
		return nil
	}

	if err := StartEve(cfg, vmName, tapInterface); err != nil {
		return fmt.Errorf("cannot start registry %s", err)
	} else {
		log.Info("Registry is running and accesible on port %d", cfg.Registry.Port)
	}
	return nil
}

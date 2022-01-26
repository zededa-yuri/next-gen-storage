package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/lf-edge/eden/pkg/utils"
	evehelper "github.com/zededa-yuri/nextgen-storage/autobench/pkg/eve"
)

type EveCommand struct {
	registryDist string `long:"registry-dist" description:"Registry dist path to store (required)"`
	vmName       string `long:"vmname" description:"vbox vmname required to create vm"`
	tapInterface string `long:"with-tap" description:"use tap interface in qemu as the third"`
}

var eveCmd EveCommand

func (x *EveCommand) Execute(args []string) error {
	configFilepath, err := utils.DefaultConfigPath()

	if err != nil {
		return fmt.Errorf("Could not load default config path %s", err)
	}

	if _, err := os.Stat(configFilepath); errors.Is(err, os.ErrNotExist) {
		utils.GenerateConfigFile(configFilepath)
	}

	eveCfg, err := evehelper.LoadConfig(configFilepath)
	if err != nil {
		return fmt.Errorf("error reading config %s", err)
	}

	currentPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error reading current working directory")
	}

	eveCfg.ConfigDir = filepath.Join(currentPath, "eve-config-dir")

	if err = evehelper.SetupEden(*eveCfg); err != nil {
		return fmt.Errorf("error setup eden %s", err)
	}
	if err = evehelper.StartEServer(*eveCfg); err != nil {
		return fmt.Errorf("error start eserver %s", err)
	}
	if err = evehelper.StartEden(*eveCfg, eveCmd.registryDist, eveCmd.vmName, eveCmd.tapInterface); err != nil {
		return fmt.Errorf("error starting eden %s", err)
	}
	if err = evehelper.OnboardEve(eveCfg.Eve.CertsUUID); err != nil {
		return fmt.Errorf("error onboarding eve %s", err)
	}

	pdInfo := evehelper.PodInfo{}
	pdInfoFile, err := ioutil.ReadFile("pdInfo.json")
	if err != nil {
		return fmt.Errorf("error opening pdInfo.json file %s", err)
	}

	if err := json.Unmarshal([]byte(pdInfoFile), &pdInfo); err != nil {
		return fmt.Errorf("error reading pod info file %s", err)
	}
	evehelper.PodDeploy(*eveCfg, pdInfo)

	return nil
}

func init() {
	parser.AddCommand(
		"eve",
		"Create eve and adam instances and runs them",
		"This command starts Test on remote machine",
		&eveCmd,
	)
}

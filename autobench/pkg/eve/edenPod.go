package evehelper

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/lf-edge/eden/pkg/controller"
	"github.com/lf-edge/eden/pkg/defaults"
	"github.com/lf-edge/eden/pkg/device"
	"github.com/lf-edge/eden/pkg/expect"
	"github.com/lf-edge/eve/api/go/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type PodInfo struct {
	PodName           string   `json:"podName"`
	NoHyper           bool     `json:"noHyper"`
	AppLink           string   `json:"appLink"`
	PodMetadata       string   `json:"podMetadata"`
	VncDisplay        uint32   `json:"vncDisplay"`
	VncPassword       string   `json:"vncPassword"`
	PodNetworkds      []string `json:"podNetworkds"`
	PortPublish       []string `json:"portPublish"`
	DiskSize          uint64   `json:"diskSize"`
	VolumeSize        uint64   `json:"volumeSize"`
	AppMemory         uint64   `json:"appMemory"`
	VolumeType        string   `json:"volumeType"`
	AppCpus           uint32   `json:"appCpus"`
	ImageFormat       string   `json:"imageFormat"`
	Acl               []string `json:"acl"`
	Vlans             []string `json:"vlans", omitempty`
	SftpLoad          bool     `json:"sftpLoad"`
	DirectLoad        bool     `json:"directLoad"`
	Mount             []string `json:"mount"`
	Disks             []string `json:"disks"`
	Registry          string   `json:"registry"`
	OpenStackMetadata bool     `json:"openStackMetadata"`
	Profiles          []string `json:"profiles"`
	DatastoreOverride string   `json:"datastoreOverride"`
	AppAdapters       []string `json:"appAdapters"`
	PodNetworks       []string `json:"podNetworks"`
	AclOnlyHost       bool     `json:"aclOnlyHost"`
}

type adamChanger struct {
	adamURL string
}

func (ctx *adamChanger) getControllerAndDev() (controller.Cloud, *device.Ctx, error) {
	ctrl, err := ctx.getController()
	if err != nil {
		return nil, nil, fmt.Errorf("getController error: %s", err)
	}
	devFirst, err := ctrl.GetDeviceCurrent()
	if err != nil {
		return nil, nil, fmt.Errorf("GetDeviceCurrent error: %s", err)
	}
	return ctrl, devFirst, nil
}

func (ctx *adamChanger) getController() (controller.Cloud, error) {
	if ctx.adamURL != "" { //overwrite config only if url defined
		ipPort := strings.Split(ctx.adamURL, ":")
		ip := ipPort[0]
		if ip == "" {
			return nil, fmt.Errorf("cannot get ip/hostname from %s", ctx.adamURL)
		}
		port := "80"
		if len(ipPort) > 1 {
			port = ipPort[1]
		}
		viper.Set("adam.ip", ip)
		viper.Set("adam.port", port)
	}
	ctrl, err := controller.CloudPrepare()
	if err != nil {
		return nil, fmt.Errorf("CloudPrepare error: %s", err)
	}
	return ctrl, nil
}

func (ctx *adamChanger) setControllerAndDev(ctrl controller.Cloud, dev *device.Ctx) error {
	if err := ctrl.ConfigSync(dev); err != nil {
		return fmt.Errorf("configSync error: %s", err)
	}
	return nil
}

func processAcls(acls []string) expect.ACLs {
	m := expect.ACLs{}
	for _, el := range acls {
		parsed := strings.SplitN(el, ":", 3)
		ni := parsed[0]
		var ep string
		if len(parsed) > 1 {
			ep = strings.TrimSpace(parsed[1])
		}
		if ep == "" {
			m[ni] = []expect.ACE{}
		} else {
			drop := false
			if len(parsed) == 3 {
				drop = parsed[2] == "drop"
			}
			m[ni] = append(m[ni], expect.ACE{Endpoint: ep, Drop: drop})
		}
	}
	return m
}

func processVLANs(vlans []string) (map[string]int, error) {
	m := map[string]int{}
	for _, el := range vlans {
		parsed := strings.SplitN(el, ":", 2)
		if len(parsed) < 2 {
			return nil, errors.New("missing VLAN ID")
		}
		vid, err := strconv.Atoi(parsed[1])
		if err != nil {
			return nil, fmt.Errorf("invalid VLAN ID: %w", err)
		}
		m[parsed[0]] = vid
	}
	return m, nil
}

func PodDeploy(cfg EdenSetupArgs, podData PodInfo) error {
	changer := &adamChanger{}
	ctrl, dev, err := changer.getControllerAndDev()
	if err != nil {
		log.Fatalf("getControllerAndDev: %s", err)
	}
	var opts []expect.ExpectationOption
	opts = append(opts, expect.WithMetadata(podData.PodMetadata))
	opts = append(opts, expect.WithVnc(podData.VncDisplay))
	opts = append(opts, expect.WithVncPassword(podData.VncPassword))
	opts = append(opts, expect.WithAppAdapters(podData.AppAdapters))
	if len(podData.PodNetworks) > 0 {
		for i, el := range podData.PodNetworks {
			if i == 0 {
				//allocate ports on first network
				opts = append(opts, expect.AddNetInstanceNameAndPortPublish(el, podData.PortPublish))
			} else {
				opts = append(opts, expect.AddNetInstanceNameAndPortPublish(el, nil))
			}
		}
	} else {
		opts = append(opts, expect.WithPortsPublish(podData.PortPublish))
	}
	diskSizeParsed, err := humanize.ParseBytes(humanize.Bytes(podData.DiskSize))
	if err != nil {
		log.Fatal(err)
	}
	opts = append(opts, expect.WithDiskSize(int64(diskSizeParsed)))
	volumeSizeParsed := podData.VolumeSize
	opts = append(opts, expect.WithVolumeSize(int64(volumeSizeParsed)))
	appMemoryParsed := podData.AppMemory

	opts = append(opts, expect.WithVolumeType(expect.VolumeTypeByName(podData.VolumeType)))
	opts = append(opts, expect.WithResources(podData.AppCpus, uint32(appMemoryParsed/1000)))
	opts = append(opts, expect.WithImageFormat(podData.ImageFormat))
	if podData.AclOnlyHost {
		opts = append(opts, expect.WithACL(map[string][]expect.ACE{
			"": {{Endpoint: defaults.DefaultHostOnlyNotation}},
		}))
	} else {
		opts = append(opts, expect.WithACL(processAcls(podData.Acl)))
	}
	vlansParsed, err := processVLANs(podData.Vlans)
	if err != nil {
		log.Fatal(err)
	}
	opts = append(opts, expect.WithVLANs(vlansParsed))
	opts = append(opts, expect.WithSFTPLoad(podData.SftpLoad))
	if !podData.SftpLoad {
		opts = append(opts, expect.WithHTTPDirectLoad(podData.DirectLoad))
	}
	opts = append(opts, expect.WithAdditionalDisks(append(podData.Disks, podData.Mount...)))
	registryToUse := podData.Registry
	switch podData.Registry {
	case "local":
		registryToUse = fmt.Sprintf("%s:%d", viper.GetString("registry.ip"), viper.GetInt("registry.port"))
	case "remote":
		registryToUse = ""
	}
	opts = append(opts, expect.WithRegistry(registryToUse))
	if podData.NoHyper {
		opts = append(opts, expect.WithVirtualizationMode(config.VmMode_NOHYPER))
	}
	opts = append(opts, expect.WithOpenStackMetadata(podData.OpenStackMetadata))
	opts = append(opts, expect.WithProfiles(podData.Profiles))
	opts = append(opts, expect.WithDatastoreOverride(podData.DatastoreOverride))
	fmt.Println("ABCDEFG %s", podData.AppLink)
	expectation := expect.AppExpectationFromURL(ctrl, dev, podData.AppLink, podData.PodName, opts...)
	appInstanceConfig := expectation.Application()
	dev.SetApplicationInstanceConfig(append(dev.GetApplicationInstances(), appInstanceConfig.Uuidandversion.Uuid))
	if err = changer.setControllerAndDev(ctrl, dev); err != nil {
		log.Fatalf("setControllerAndDev: %s", err)
	}
	log.Infof("deploy pod %s with %s request sent", appInstanceConfig.Displayname, podData.AppLink)
	return nil
}

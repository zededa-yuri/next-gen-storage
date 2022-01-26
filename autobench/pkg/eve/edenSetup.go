package evehelper

import (
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lf-edge/eden/pkg/defaults"
	"github.com/lf-edge/eden/pkg/eden"
	"github.com/lf-edge/eden/pkg/models"
	"github.com/lf-edge/eden/pkg/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

func generateScripts(in string, out string, configFile string) {
	tmpl, err := ioutil.ReadFile(in)
	if err != nil {
		log.Fatal(err)
	}
	script, err := utils.RenderTemplate(configFile, string(tmpl))
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(out, []byte(script), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func SetupEden(cfg EdenSetupArgs) error {

	model, err := models.GetDevModelByName(cfg.Eve.DevModel)
	if err != nil {
		log.Fatalf("GetDevModelByName: %s", err)
	}
	if cfg.Netboot && cfg.Installer {
		log.Fatal("Please use netboot or installer flag, not both")
	}
	if cfg.Netboot || cfg.Installer {
		if cfg.Eve.DevModel != defaults.DefaultGeneralModel {
			log.Fatalf("Cannot use netboot for devmodel %s, please use general instead", cfg.Eve.DevModel)
		}
	}
	if cfg.Eve.DevModel == defaults.DefaultQemuModel {
		if _, err := os.Stat(cfg.Eve.QemuFileToSave); os.IsNotExist(err) {
			f, err := os.Create(cfg.Eve.QemuFileToSave)
			if err != nil {
				log.Fatal(err)
			}
			qemuDTBPathAbsolute := ""
			if cfg.Eve.QemuDTBPath != "" {
				qemuDTBPathAbsolute, err = filepath.Abs(cfg.Eve.QemuDTBPath)
				if err != nil {
					log.Fatal(err)
				}
			}
			var qemuFirmwareParam []string
			for _, line := range cfg.Eve.QemuFirmware {
				for _, el := range strings.Split(line, " ") {
					qemuFirmwareParam = append(qemuFirmwareParam, utils.ResolveAbsPath(el))
				}
			}
			settings := utils.QemuSettings{
				DTBDrive: qemuDTBPathAbsolute,
				Firmware: qemuFirmwareParam,
				MemoryMB: cfg.Eve.QemuMemory,
				CPUs:     cfg.Eve.QemuCpus,
			}
			conf, err := settings.GenerateQemuConfig()
			if err != nil {
				log.Fatal(err)
			}
			_, err = f.Write(conf)
			if err != nil {
				log.Fatal(err)
			}
			if err := f.Close(); err != nil {
				log.Fatal(err)
			}
			log.Infof("QEMU config file generated: %s", cfg.Eve.QemuFileToSave)
		} else {
			log.Debugf("QEMU config already exists: %s", cfg.Eve.QemuFileToSave)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.Eden.CertsDir, "root-certificate.pem")); os.IsNotExist(err) {
		wifiPSK := ""
		if cfg.Eve.Ssid != "" {
			fmt.Printf("Enter password for wifi %s: ", cfg.Eve.Ssid)
			pass, _ := term.ReadPassword(0)
			wifiPSK = strings.ToLower(hex.EncodeToString(pbkdf2.Key(pass, []byte(cfg.Eve.Ssid), 4096, 32, sha1.New)))
			fmt.Println()
		}
		if cfg.ZedcontrolURL == "" {
			if err := eden.GenerateEveCerts(cfg.Eden.CertsDir, cfg.Adam.CertsDomain, cfg.Adam.CertsIP, cfg.Adam.CertsEVEIP, cfg.Eve.CertsUUID, cfg.Eve.DevModel, cfg.Eve.Ssid, wifiPSK, cfg.Adam.ApiV1); err != nil {
				return fmt.Errorf("cannot GenerateEveCerts: %s", err)
			} else {
				log.Info("GenerateEveCerts done")
			}
		} else {
			if err := eden.PutEveCerts(cfg.Eden.CertsDir, cfg.Eve.DevModel, cfg.Eve.Ssid, wifiPSK); err != nil {
				return fmt.Errorf("cannot GenerateEveCerts: %s", err)
			} else {
				log.Info("GenerateEveCerts done")
			}
		}
	} else {
		log.Info("GenerateEveCerts done")
		log.Infof("Certs already exists in certs dir: %s", cfg.Eden.CertsDir)
	}

	if cfg.ZedcontrolURL == "" {
		if err := eden.GenerateEVEConfig(cfg.Eden.CertsDir, cfg.Adam.CertsDomain, cfg.Adam.CertsEVEIP, cfg.Adam.Port, cfg.Adam.ApiV1, cfg.Softserial); err != nil {
			return fmt.Errorf("cannot GenerateEVEConfig: %s", err)
		} else {
			log.Info("GenerateEVEConfig done")
		}
	} else {
		if err := eden.GenerateEVEConfig(cfg.Eden.CertsDir, cfg.ZedcontrolURL, "", 0, false, cfg.Softserial); err != nil {
			return fmt.Errorf("cannot GenerateEVEConfig: %s", err)
		} else {
			log.Info("GenerateEVEConfig done")
		}
	}
	if _, err := os.Lstat(cfg.ConfigDir); !os.IsNotExist(err) {
		//put files from config folder to generated directory
		if err := utils.CopyFolder(utils.ResolveAbsPath(cfg.EveConfigDir), cfg.Eden.CertsDir); err != nil {
			return fmt.Errorf("CopyFolder: %s", err)
		}
	}
	imageFormat := model.DiskFormat()
	if !cfg.Eden.Download {
		if _, err := os.Lstat(cfg.Eve.ImageFile); os.IsNotExist(err) {
			if err := eden.CloneFromGit(cfg.Eve.Dist, cfg.Eve.Repo, cfg.Eve.Tag); err != nil {
				return fmt.Errorf("cannot clone EVE: %s", err)
			} else {
				log.Info("clone EVE done")
			}
			builedImage := ""
			builedAdditional := ""
			if builedImage, builedAdditional, err = eden.MakeEveInRepo(cfg.Eve.Dist, cfg.Eden.CertsDir, cfg.Eve.Arch, cfg.Eve.HV, imageFormat, false); err != nil {
				return fmt.Errorf("cannot MakeEveInRepo: %s", err)
			} else {
				log.Info("MakeEveInRepo done")
			}
			if err = utils.CopyFile(builedImage, cfg.Eve.ImageFile); err != nil {
				log.Fatal(err)
			}
			builedAdditionalSplitted := strings.Split(builedAdditional, ",")
			for _, additionalFile := range builedAdditionalSplitted {
				if additionalFile != "" {
					if err = utils.CopyFile(additionalFile, filepath.Join(filepath.Dir(cfg.Eve.ImageFile), filepath.Base(additionalFile))); err != nil {
						log.Fatal(err)
					}
				}
			}
			log.Infof(model.DiskReadyMessage(), cfg.Eve.ImageFile)
		} else {
			log.Infof("EVE already exists in dir: %s", cfg.Eve.Dist)
		}
	} else {
		eveDesc := utils.EVEDescription{
			ConfigPath:  cfg.Eden.CertsDir,
			Arch:        cfg.Eve.Arch,
			HV:          cfg.Eve.HV,
			Registry:    cfg.Eve.Registry,
			Tag:         cfg.Eve.Tag,
			Format:      imageFormat,
			ImageSizeMB: cfg.Eve.ImageSizeMB,
		}
		uefiDesc := utils.UEFIDescription{
			Registry: cfg.Eve.Registry,
			Tag:      cfg.Eve.UefiTag,
			Arch:     cfg.Eve.Arch,
		}
		imageTag, err := eveDesc.Image()
		if err != nil {
			log.Fatal(err)
		}
		if cfg.Netboot {
			if err := utils.DownloadEveNetBoot(eveDesc, filepath.Dir(cfg.Eve.ImageFile)); err != nil {
				return fmt.Errorf("cannot download EVE: %s", err)
			} else {
				if err := eden.StartEServer(cfg.Eden.Eserver.Port, cfg.Eden.Images.EserverImageDist, cfg.Eden.Eserver.Force, cfg.Eden.Eserver.Tag); err != nil {
					return fmt.Errorf("cannot start eserver: %s", err)
				} else {
					log.Infof("Eserver is running and accessible on port %d", cfg.Eden.Eserver.Port)
				}
				eServerIP := cfg.Adam.CertsEVEIP
				eServerPort := cfg.Eden.Eserver.Port

				server := &eden.EServer{
					EServerIP:   eServerIP,
					EServerPort: string(eServerPort),
				}
				// we should uncompress kernel for arm64
				if cfg.Eve.Arch == "arm64" {
					// rename to temp file
					if err := os.Rename(filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "kernel"),
						filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "kernel.old")); err != nil {
						// probably naming changed, give up
						log.Warnf("Cannot rename kernel: %v", err)
					} else {
						r, err := os.Open(filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "kernel.old"))
						if err != nil {
							log.Fatalf("Open kernel.old: %v", err)
						}
						uncompressedStream, err := gzip.NewReader(r)
						if err != nil {
							// in case of non-gz rename back
							return fmt.Errorf("gzip: NewReader failed: %v", err)
							if err := os.Rename(filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "kernel.old"),
								filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "kernel")); err != nil {
								log.Fatalf("Cannot rename kernel: %v", err)
							}
						} else {
							defer uncompressedStream.Close()
							out, err := os.Create(filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "kernel"))
							if err != nil {
								log.Fatalf("Cannot create file to save: %v", err)
							}
							if _, err := io.Copy(out, uncompressedStream); err != nil {
								log.Fatalf("Cannot copy to decompressed file: %v", err)
							}
							if err := out.Close(); err != nil {
								log.Fatalf("Cannot close file: %v", err)
							}
						}
					}
				}
				items, _ := ioutil.ReadDir(filepath.Dir(cfg.Eve.ImageFile))
				for _, item := range items {
					if !item.IsDir() && item.Name() != "ipxe.efi.cfg" {
						if _, err := eden.AddFileIntoEServer(server, filepath.Join(filepath.Dir(cfg.Eve.ImageFile), item.Name())); err != nil {
							log.Fatalf("AddFileIntoEServer: %s", err)
						}
					}
				}
				ipxeFile := filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "ipxe.efi.cfg")
				ipxeFileBytes, err := ioutil.ReadFile(ipxeFile)
				if err != nil {
					log.Fatalf("Cannot read ipxe file: %v", err)
				}
				re := regexp.MustCompile("# set url .*")
				ipxeFileReplaced := re.ReplaceAll(ipxeFileBytes,
					[]byte(fmt.Sprintf("set url http://%s:%d/%s/", eServerIP, cfg.Eden.Eserver.Port, "eserver")))
				if cfg.Softserial != "" {
					ipxeFileReplaced = []byte(strings.ReplaceAll(string(ipxeFileReplaced),
						"eve_soft_serial=${mac:hexhyp}",
						fmt.Sprintf("eve_soft_serial=%s", cfg.Softserial)))
				}
				_ = os.MkdirAll(filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "tftp"), 0777)
				ipxeConfigFile := filepath.Join(filepath.Dir(cfg.Eve.ImageFile), "tftp", "ipxe.efi.cfg")
				_ = ioutil.WriteFile(ipxeConfigFile, ipxeFileReplaced, 0777)
				if _, err := eden.AddFileIntoEServer(server, ipxeConfigFile); err != nil {
					log.Fatalf("AddFileIntoEServer: %s", err)
				}
				log.Infof("download EVE done: %s", imageTag)
				log.Infof("Please use %s to boot your EVE via ipxe", ipxeConfigFile)
				log.Infof("ipxe.efi.cfg uploaded to eserver (http://%s:%s/eserver/ipxe.efi.cfg). Use it to boot your EVE via network", eServerIP, eServerPort)
				log.Infof("EVE already exists: %s", filepath.Dir(cfg.Eve.ImageFile))
			}
		} else if cfg.Installer {
			if _, err := os.Lstat(cfg.Eve.ImageFile); os.IsNotExist(err) {
				if err := utils.DownloadEveInstaller(eveDesc, cfg.Eve.ImageFile); err != nil {
					return fmt.Errorf("cannot download EVE: %s", err)
				} else {
					log.Infof("download EVE done: %s", imageTag)
					log.Infof(model.DiskReadyMessage(), cfg.Eve.ImageFile)
				}
			} else {
				log.Infof("download EVE done: %s", imageTag)
				log.Infof("EVE already exists: %s", cfg.Eve.ImageFile)
			}
		} else {
			if _, err := os.Lstat(cfg.Eve.ImageFile); os.IsNotExist(err) {
				if err := utils.DownloadEveLive(eveDesc, uefiDesc, cfg.Eve.ImageFile); err != nil {
					return fmt.Errorf("cannot download EVE: %s", err)
				} else {
					log.Infof("download EVE done: %s", imageTag)
					log.Infof(model.DiskReadyMessage(), cfg.Eve.ImageFile)
				}
			} else {
				log.Infof("download EVE done: %s", imageTag)
				log.Infof("EVE already exists: %s", cfg.Eve.ImageFile)
			}
		}

		if cfg.ZedcontrolURL != "" {
			log.Printf("Please use %s as Onboarding Key", defaults.OnboardUUID)
			if cfg.Softserial != "" {
				log.Printf("use %s as Serial Number", cfg.Softserial)
			}
			log.Printf("To onboard EVE onto %s", cfg.ZedcontrolURL)
		}
	}
	return nil
}

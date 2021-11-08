package fiotests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fioconv"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/sshwork"
	"golang.org/x/crypto/ssh"
)

func RunFIOTest(client *ssh.Client, sshUser, localResultsFolder, localDirResults, targetDevice string, fioOptions mkconfig.FioOptions, fioTestTime time.Duration) error {
	curentDate := fmt.Sprintf(time.Now().Format("2006-01-02-15:04:05"))

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	exPath := filepath.Dir(ex)

	// Create folder for results
	localResultsAbsDir := localDirResults
	if localResultsAbsDir == "" {
		localResultsAbsDir = filepath.Join(exPath, localResultsFolder + curentDate)
		err = os.Mkdir(localResultsAbsDir, 0755)
		if err != nil {
			return fmt.Errorf("could not create local dir for result: %w", err)
		}
	}

	// Check FIO tools on VM
	if err := sshwork.SendCommandSSH(
		client,
		"fio -h",
		true, // sshwork.Foreground - true | sshwork.Background - false
	); err != nil {
		return fmt.Errorf("FIO tools not found on VM: %w", err)
	}

	// Check free space on VM
	// Fix me ^

	// Create folder on VM
	remoteResultsAbsDir := filepath.Join("/home/", sshUser, "/FIO" + curentDate)
	if err := sshwork.SendCommandSSH(
		client,
		fmt.Sprintf("mkdir %s", remoteResultsAbsDir),
		true, // sshwork.Foreground - true | sshwork.Background - false
	); err != nil {
		return fmt.Errorf("could not create remote dir for result: %w", err)
	}

	remoteResultsAbsDirLogs := filepath.Join(remoteResultsAbsDir, "logs")
	if err := sshwork.SendCommandSSH(
		client,
		fmt.Sprintf("mkdir %s", remoteResultsAbsDirLogs),
		true, // sshwork.Foreground - true | sshwork.Background - false
	); err != nil {
		return fmt.Errorf("could not create remote dir for log-result: %w", err)
	}

	// Create config for fio
	localFioConfig := filepath.Join(localResultsAbsDir, "fio_config.cfg")
	if err := mkconfig.GenerateFIOConfig(
		fioOptions,
		fioTestTime,
		localFioConfig,
		sshUser,
		targetDevice,
		remoteResultsAbsDirLogs,
	); err != nil {
		return fmt.Errorf("create fio config failed: %w", err)
	}

	if err := sshwork.SendFileSCP(
		client,
		localFioConfig,
		filepath.Join(remoteResultsAbsDir, "/fio_config.cfg"),
	); err != nil {
		return fmt.Errorf("could not send file to VM: %w", err)
	}

	// Waiting end fio test
	var countTests = mkconfig.CountTests(fioOptions)
	const bufferTime = 50 * time.Second
	var totalTime = time.Duration(int64(countTests) * int64(fioTestTime) + int64(bufferTime))

	// Run fio test  [!WE NEED SUDO PRIVILEGES HERE]
	fioRunCmd := fmt.Sprintf(
		"sudo fio %s --output-format=normal,json > %s & ",
		filepath.Join(remoteResultsAbsDir, "/fio_config.cfg"),
		filepath.Join(remoteResultsAbsDir, "/result.json"),
	)

	go func() {
		if err := sshwork.SendCommandSSH(client, fioRunCmd, true); err != nil {
			fmt.Println("FIO test failed (maybe we need sudo): %w", err)
		}
	}()

	// Heartbeat
	timerTomeOut := time.After(totalTime)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	there:
	for {
		select {
		case <-timerTomeOut:
			ticker.Stop()
			break there
		case <- ticker.C:
			if err := sshwork.SendCommandSSH(client, "pgrep fio", true); err != nil {
				return fmt.Errorf("VM is fail. Test failed FIO process on VM not found")
			}
			fmt.Println("Checking... Nothing broken yet. Let's wait a bit. ")
		}
	}

	// Download fio reults
	fmt.Println("Downloading the results ...")
	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/result.json"),
		filepath.Join(remoteResultsAbsDir, "/result.json"),
	); err != nil {
		return fmt.Errorf("could not get result.json file from VM: %w", err)
	}

	if err := fioconv.ConvertJSONtoCSV(
		filepath.Join(localResultsAbsDir, "/result.json"),
		filepath.Join(localResultsAbsDir, "/FIOresult.csv"),
	); err != nil {
		return fmt.Errorf("could not convert JSON to CSV: %w", err)
	}

	if err := sshwork.SendCommandSSH(
		client,
		fmt.Sprintf("tar -czvf %s %s", filepath.Join(remoteResultsAbsDir, "logs.tar.gz"), remoteResultsAbsDirLogs),
		true, // sshwork.Foreground - true | sshwork.Background - false
	); err != nil {
		return fmt.Errorf("could not create remote arhive logs.tar.gz: %w", err)
	}

	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/logs.tar.gz"),
		filepath.Join(remoteResultsAbsDir, "/logs.tar.gz"),
	); err != nil {
		return fmt.Errorf("could not get logs.tar.gz file from VM: %w", err)
	}
	fmt.Println("The download of the results for was successful")

	// Download remote dmesg reults cat
	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/guest_dmesg"),
		"/var/log/dmesg",
	); err != nil {
		if err := sshwork.GetFileSCP(
			client,
			filepath.Join(localResultsAbsDir, "/guest_dmesg"),
			"/var/log/kern.log",
		); err != nil {
			fmt.Println("could not get dmesg file from VM: ", err)
		}
	}

	// Save local dmesg file
	out, err := exec.Command("cp", "/var/log/dmesg",
							filepath.Join(localResultsAbsDir, "/host_dmesg"),
							).CombinedOutput()
	if err != nil {
		fmt.Println("copying local dmesg file with logs failed! ", err, out)
	}

	// Saving information about the hardware
	output, err := exec.Command("lshw").CombinedOutput()
	if err != nil {
		fmt.Println("failed to collect hardware data! ", err)
	} else {
		lshw := filepath.Join(localResultsAbsDir, "lshw-result")
		file, err := os.Create(lshw)
    	if err != nil{
    	    fmt.Println("failed to create file with hardware information: ", err)
    	}
    	defer file.Close()
    	file.WriteString(string(output))
	}

	return nil
}

func RunFIOTestLocal(user, localResultsFolder, localDirResults, targetDevice string,
					fioOptions mkconfig.FioOptions, fioTestTime time.Duration) error {
	curentDate := fmt.Sprintf(time.Now().Format("2006-01-02-15:04:05"))

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	exPath := filepath.Dir(ex)

	// Create folder for results
	localResultsAbsDir := localDirResults
	if localResultsAbsDir == "" {
		localResultsAbsDir = filepath.Join(exPath, localResultsFolder + curentDate)
		err = os.Mkdir(localResultsAbsDir, 0755)
		if err != nil {
			return fmt.Errorf("could not create local dir for result: %w", err)
		}
	}

	localResultsAbsDirLogs := filepath.Join(localResultsAbsDir, "logs")
	err = os.Mkdir(localResultsAbsDirLogs, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for log-result: %w", err)
	}

	// Create config for fio
	localFioConfig := filepath.Join(localResultsAbsDir, "fio_config.cfg")
	mkconfig.GenerateFIOConfig(fioOptions, fioTestTime, localFioConfig, user, targetDevice, localResultsAbsDir)

	// Waiting end fio test
	var countTests = mkconfig.CountTests(fioOptions)
	const bufferTime = 2 * time.Minute
	var totalTime = time.Duration(int64(countTests) * int64(fioTestTime) + int64(bufferTime))

	go func() {
		_, err := exec.Command("fio", filepath.Join(localResultsAbsDir, "/fio_config.cfg"),
								"--output-format=normal,json", ">>",
								filepath.Join(localResultsAbsDir,
								"/result.json")).CombinedOutput()
		if err != nil {
			fmt.Println("Failed to exec FIO command! ", err)
		}
	}()

	// Heartbeat
	timerTomeOut := time.After(totalTime)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	there:
	for {
		select {
		case <-timerTomeOut:
			ticker.Stop()
			break there
		case <- ticker.C:
			_, err := exec.Command("pgrep", "fio").CombinedOutput()
			if err != nil {
				fmt.Println("Test failed! FIO process on local machine not found! ", err)
				break there
			}
			fmt.Println("Checking... Nothing broken yet. Let's wait a bit. ")
		}
	}
	if err != nil {
		return fmt.Errorf("FIO test failed: %w", err)
	}

	if err := fioconv.ConvertJSONtoCSV(
		filepath.Join(localResultsAbsDir, "/result.json"),
		filepath.Join(localResultsAbsDir, "/FIOresult.csv"),
	); err != nil {
		return fmt.Errorf("could not convert JSON to CSV: %w", err)
	}

	// Save local dmesg file
	out, err := exec.Command("cp", "/var/log/dmesg", filepath.Join(localResultsAbsDir, "/host_dmesg")).CombinedOutput()
	if err != nil {
		fmt.Println("Copying local dmesg file with logs failed! ", err, out)
	}

	// Saving information about the hardware
	output, err := exec.Command("lshw").CombinedOutput()
	if err != nil {
		fmt.Println("Failed to collect hardware data! ", err)
	}
	lshw := filepath.Join(localResultsAbsDir, "lshw-result")
	file, err := os.Create(lshw)
    if err != nil{
        fmt.Println("Failed to create file with hardware information: ", err)
    }
    defer file.Close()
    file.WriteString(string(output))

	fmt.Println("Tests finished!")
	return nil
}

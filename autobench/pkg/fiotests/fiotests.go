package fiotests

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fioconv"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/sshwork"
	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
)


func SshConnect(ip, user string) (*ssh.Client, error) {
	home := os.Getenv("HOME")
	key_path := fmt.Sprintf("%s/.ssh/id_rsa", home)

	log.Printf("Loading keyfile %s\n", key_path)
	key, err := ioutil.ReadFile(key_path)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	known_hosts_path := fmt.Sprintf("%s/.ssh/known_hosts", home)
	hostKeyCallback, err := kh.New(known_hosts_path)
	if err != nil {
		return nil, fmt.Errorf("could not create hostkeycallback function: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", ip, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect: %w", err)
	}
	return client, nil
}

func RunFIOTest(sshHost, sshUser, localResultsFolder, localDirResults, targetDevice string, fioOptions mkconfig.FioOptions, fioTestTime time.Duration) error {
	curentDate := fmt.Sprintf(time.Now().Format("2006-01-02-15:04:05"))
	// Get ssh client
	client, err := SshConnect(sshHost, sshUser)
	if err != nil {
		return fmt.Errorf("unable to connect: %v", err)
	}
	defer client.Close()

	// Install tools on remote VM [!WE NEED SUDO PRIVILEGES HERE]
	if err := sshwork.SendCommandSSH(
		client,
		"apt-get update && apt-get install -y fio git lshw sysstat",
		true, // sshwork.Foreground - true | sshwork.Background - false
	); err != nil {
		// Not critical
		//return fmt.Errorf("couldnot install tools on VM(maybe we need sudo): %w", err)
	}

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println("exPath:", exPath)

	// Create folder for results
	localResultsAbsDir := localDirResults
	if localResultsAbsDir == "" {
		localResultsAbsDir = filepath.Join(exPath, localResultsFolder + curentDate)
	}
	err = os.Mkdir(localResultsAbsDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	// Create config for fio
	localFioConfig := filepath.Join(localResultsAbsDir, "fio_config.cfg")
	mkconfig.GenerateFIOConfig(fioOptions, fioTestTime, localFioConfig, sshUser, targetDevice)

	// Create folder on VM  FIXME
	remoteResultsAbsDir := filepath.Join("/users/", sshUser, "/FIO" + curentDate)
	if err := sshwork.SendCommandSSH(
		client,
		fmt.Sprintf("mkdir %s", remoteResultsAbsDir),
		true, // sshwork.Foreground - true | sshwork.Background - false
	); err != nil {
		return fmt.Errorf("couldnot install tools on VM: %w", err)
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
	const bufferTime = 2 * time.Minute
	var totalTime = time.Duration(int64(countTests) * int64(fioTestTime) + int64(bufferTime))

	// Run fio test  [!WE NEED SUDO PRIVILEGES HERE]
	fioRunCmd := fmt.Sprintf(
		"fio %s --output-format=normal,json > %s & ",
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
				return fmt.Errorf("VM is fail. Test failed")
			}
			fmt.Println("Checking... Nothing broken yet. Let's wait a bit. ", sshHost)
		}
	}

	// Download fio reults
	fmt.Println("Downloading the results ...")
	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/result.json"),
		filepath.Join(remoteResultsAbsDir, "/result.json"),
	); err != nil {
		return fmt.Errorf("Could not get result.json file from VM: %w", err)
	}

	// Download remote dmesg reults
	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/guest_dmesg"),
		"/var/log/dmesg",
	); err != nil {
		return fmt.Errorf("could not get dmesg file from VM: %w", err)
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

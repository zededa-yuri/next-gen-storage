package fiotests

import (
	"io/ioutil"
	"log"
	"os"
	"fmt"
	"time"
	"path/filepath"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/sshwork"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fioconv"
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

func RunFIOTest(sshHost, sshUser, localResultsDir string, fioOptions mkconfig.FioOptions, fioTestTime time.Duration) error {
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
		return fmt.Errorf("couldnot install tools on VM(maybe we need sudo): %w", err)
	}

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	exPath := filepath.Dir(ex)
	fmt.Println("exPath:", exPath)

	// Create folder for results
	localResultsAbsDir := filepath.Join(exPath, localResultsDir)
	err = os.Mkdir(localResultsAbsDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	// Create config for fio
	localFioConfig := filepath.Join(localResultsAbsDir, "fio_config.cfg")
	mkconfig.GenerateFIOConfig(fioOptions, fioTestTime, localFioConfig)

	// Create folder on VM
	remoteResultsAbsDir := filepath.Join("/users/", sshUser, "/FIO")
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

	// Run fio test  [!WE NEED SUDO PRIVILEGES HERE]
	fioRunCmd := fmt.Sprintf(
		`fio %s --output-format=normal,json > %s &`,
		filepath.Join(remoteResultsAbsDir, "/fio_config.cfg"),
		filepath.Join(remoteResultsAbsDir, "/result.json"),
	)

	if err := sshwork.SendCommandSSH(client, fioRunCmd, false); err != nil {
		return fmt.Errorf("FIO test failed (maybe we need sudo): %w", err)
	}
	// Waiting end fio test
	var countTests = mkconfig.CountTests(fioOptions)
	const bufferTime = 2 * time.Minute
	var totalTime = time.Duration(int64(countTests) * int64(fioTestTime) + int64(bufferTime))

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
			if err := sshwork.SendCommandSSH(client, "true", true); err != nil {
				return fmt.Errorf("VM is fail. Test failed")
			}
			fmt.Println("Checking... Nothing broken yet. Let's wait a bit.")
		}
	}

	// Download fio reults
	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/result.json"),
		filepath.Join(remoteResultsAbsDir, "/result.json"),
	); err != nil {
		return fmt.Errorf("could not get result.json file from VM: %w", err)
	}

	// Download dmesg reults
	if err := sshwork.GetFileSCP(
		client,
		filepath.Join(localResultsAbsDir, "/dmesg"),
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

	return nil
}

/*
func main() {

}
 */

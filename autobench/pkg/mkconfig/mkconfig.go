package mkconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const globalTpl = `[global]
ioengine=libaio
size=%dG
direct=1
runtime=%s
time_based=1
group_reporting=1
filename=%s
log_avg_msec=250

`

const globalTplcheckSumm = `[global]
ioengine=libaio
size=%dG
direct=1
runtime=%s
verify=%s
verify_fatal=1
time_based=1
group_reporting=1
filename=%s
log_avg_msec=250

`

const sectionTpl = `
[%s]
rw=%s
bs=%s
iodepth=%d
numjobs=%d
write_bw_log=%s
write_iops_log=%s
write_lat_log=%s
stonewall
`

func Contains(hs []string, val string) bool {
	for _, v := range hs {
		if v == val {
			return true
		}
	}
	return false
}

func containsInt(hs []int, val int) bool {
	for _, v := range hs {
		if v == val {
			return true
		}
	}
	return false
}

type OpType []string

func (t *OpType) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	v = strings.ToLower(v)
	var val = strings.Split(v, ",")
	*t = []string{}

	var valid = []string{"read", "write", "randread", "randwrite", "trim", "randtrim"}
	for _, s := range val {
		if !Contains(valid, s) {
			return fmt.Errorf("Invalid value for operation type: %s\n\tUse something from this list: %v\n", s, valid)
		}
		*t = append(*t, s)
	}

	return nil
}

type BSType []string

func (bs *BSType) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	v = strings.ToLower(v)
	var val = strings.Split(v, ",")
	var valid = []string{"512", "1k", "2k", "4k", "8k", "16k", "32k", "64k", "128k", "256k", "512k", "1m"}
	*bs = []string{}
	for _, s := range val {
		if !Contains(valid, s) {
			return fmt.Errorf("Invalid value for block size: %s\n\tUse something from this list: %v\n", s, valid)
		}
		*bs = append(*bs, s)
	}

	return nil
}

type JobsType []int

func (j *JobsType) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	v = strings.ToLower(v)
	var val = strings.Split(v, ",")
	*j = []int{}
	var valid = []int{1, 4, 8, 16, 32}
	for _, s := range val {
		n, err := strconv.Atoi(s)
		if err != nil || !containsInt(valid, n) {
			return fmt.Errorf("Invalid value for jobs %d\n\tUse something from this list: %v\n", n, valid)
		}
		*j = append(*j, n)
	}

	return nil
}

type DepthType []int

func (d *DepthType) Set(v string) error {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	v = strings.ToLower(v)
	var val = strings.Split(v, ",")
	*d = []int{}
	var valid = []int{1, 4, 8, 16, 32, 64}
	for _, s := range val {
		n, err := strconv.Atoi(s)
		if err != nil || !containsInt(valid, n) {
			return fmt.Errorf("Invalid value for iodepth %d\n\tUse something from this list: %v\n", n, valid)
		}
		*d = append(*d, n)
	}

	return nil
}
type FioOptions struct {
	Operations 	OpType
	BlockSize   BSType
	Jobs      	JobsType
	Iodepth     DepthType
	CheckSumm	string
	SizeGb      int
}

func CountTests(cfg FioOptions) int {
	if len(cfg.Operations) == 0 {
		cfg.Operations = OpType{"read", "write"}
	}

	if len(cfg.BlockSize) == 0 {
		cfg.BlockSize = BSType{"4k", "64k", "1m"}
	}

	if len(cfg.Jobs) == 0 {
		cfg.Jobs = JobsType{1, 8}
	}

	if len(cfg.Iodepth) == 0 {
		cfg.Iodepth = DepthType{1, 8, 32}
	}
	return len(cfg.Operations) * len(cfg.BlockSize) * len(cfg.Jobs) * len(cfg.Iodepth)
}

// GenerateFIOConfig generate confiig for FIO from provided params
// and outputs to file `outPath`.
// File will be overwriten if it is already exists
func GenerateFIOConfig(
	cfg FioOptions,
	runtime time.Duration,
	outPath, sshUser, targetDevice, remoteDirResults string,
) error {
	if len(cfg.Operations) == 0 {
		cfg.Operations = OpType{"read", "write"}
	}

	if len(cfg.BlockSize) == 0 {
		cfg.BlockSize = BSType{"4k", "64k", "1m"}
	}

	if len(cfg.Jobs) == 0 {
		cfg.Jobs = JobsType{1, 8}
	}

	if len(cfg.Iodepth) == 0 {
		cfg.Iodepth = DepthType{1, 8, 32}
	}

	if (runtime == 0) {
		runtime = 60 * time.Second
	}

	if (cfg.SizeGb == 0) {
		cfg.SizeGb = 1
	}

	var sTime = fmt.Sprintf("%d", int64(runtime.Round(time.Second).Seconds()))

	ftPath := filepath.Join("/home/", sshUser, "/fio.test.file")
	if targetDevice != "" {
		ftPath = targetDevice
	}

	fd, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("could not create output file [%s]: %w", outPath, err)
	}
	defer fd.Close()

	if cfg.CheckSumm != "" {
		fmt.Fprintf(fd, globalTplcheckSumm, cfg.SizeGb, sTime, cfg.CheckSumm, ftPath)
	} else {
		fmt.Fprintf(fd, globalTpl, cfg.SizeGb, sTime, ftPath)
	}

	for _, rw := range cfg.Operations {
		for _, bs := range cfg.BlockSize {
			var count = 0
			for _, depth := range cfg.Iodepth {
				for _, job := range cfg.Jobs {
					var section = fmt.Sprintf("%s-%s-%d", rw, bs, count)
					var logResName = filepath.Join(remoteDirResults, section)
					fmt.Fprintf(fd, sectionTpl, section, rw, bs, depth, job, logResName, logResName, logResName)
					count++
				}
			}
		}
	}

	return nil
}

package mkconfig

import (
	"fmt"
	"time"
	"os"
)

const globalTpl = `[global]
ioengine=libaio
size=1G
direct=1
runtime=%s
time_based=1
group_reporting
filename=%s
`

const globalTplcheckSumm = `[global]
ioengine=libaio
size=1G
direct=1
runtime=%s
verify=%s
verify_fatal=1
time_based=1
group_reporting
filename=%s
`

const sectionTpl = `
[%s]
rw=%s
bs=%s
iodepth=%d
numjobs=%d
stonewall
`

// Type of I/O pattern.
type OperationType string
const (
	// Sequential reads.
	OperationTypeRead OperationType = "read"
	// Sequential writes.
	OperationTypeWrite OperationType = "write"
	// Sequential mixed reads and writes.
	OperationTypeReadWrite OperationType = "readwrite"
	// Random reads.
	OperationTypeRandRead OperationType = "randread"
	// Random writes.
	OperationTypeRandWrite OperationType = "randwrite"
	// Random mixed reads and writes.
	OperationTypeRandReadWrite OperationType = "randrw"
)

// The block size in bytes used for I/O units.
type BlockSize string
const (
	BlockSize512 BlockSize = "512"
	BlockSize1k BlockSize = "1k"
	BlockSize2k BlockSize = "2k"
	BlockSize4k BlockSize = "4k"
	BlockSize8k BlockSize = "8k"
	BlockSize16k BlockSize = "16k"
	BlockSize32k BlockSize = "32k"
	BlockSize64k BlockSize = "64k"
	BlockSize128k BlockSize = "128k"
	BlockSize256k BlockSize = "256k"
	BlockSize512k BlockSize = "512k"
	BlockSize1m BlockSize = "1m"
)

// Create the specified number of clones of this job.
type JobType int
const (
	JobType1 JobType = 1
	JobType4 JobType = 4
	JobType8 JobType = 8
	JobType16 JobType = 16
	JobType32 JobType = 32
)

// Number of I/O units to keep in flight against the file.
type DepthType int
const (
	DepthType1 DepthType = 1
	DepthType4 DepthType = 4
	DepthType8 DepthType = 8
	DepthType16 DepthType = 16
	DepthType32 DepthType = 32
)

type FioOptions struct {
	OperationType []OperationType
	BlockSize     []BlockSize
	JobType       []JobType
	DepthType     []DepthType
}

func CountTests(cfg FioOptions) int {
	return len(cfg.OperationType) * len(cfg.BlockSize) * len(cfg.JobType) * len(cfg.DepthType)
}

// GenerateFIOConfig generate confiig for FIO from provided params
// and outputs to file `outPath`.
// File will be overwriten if it is already exists
func GenerateFIOConfig(
	cfg FioOptions,
	runtime time.Duration,
	outPath string,
) error {
	if len(cfg.OperationType) == 0 {
		cfg.OperationType = []OperationType{
			OperationTypeRandRead,
			OperationTypeRandWrite,
		}
	}

	if len(cfg.BlockSize) == 0 {
		cfg.BlockSize = []BlockSize{
			BlockSize4k,
			BlockSize64k,
			BlockSize1m,
		}
	}

	if len(cfg.JobType) == 0 {
		cfg.JobType = []JobType{
			JobType1,
			JobType8,
		}
	}

	if len(cfg.DepthType) == 0 {
		cfg.DepthType = []DepthType{
			DepthType1,
			DepthType8,
			DepthType32,
		}
	}

	if (runtime == 0) {
		runtime = 60 * time.Second
	}
	var sTime = fmt.Sprintf("%d", int64(runtime.Round(time.Second).Seconds()))

	var countTests = CountTests(cfg)
	const ftPath = "/fio.test.file"
 	fmt.Fprintln(os.Stderr, "type:", cfg.OperationType)
	fmt.Fprintln(os.Stderr, "bs:", cfg.BlockSize)
	fmt.Fprintln(os.Stderr, "jobs:", cfg.JobType)
	fmt.Fprintln(os.Stderr, "depth:", cfg.DepthType)
	fmt.Fprintln(os.Stderr, "time:", sTime)
	fmt.Fprintln(os.Stderr, "Total tests:", countTests)

	fd, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("could not create output file [%s]: %w", outPath, err)
	}
	defer fd.Close()

	// verify, exists := os.LookupEnv("FIO_CHECKSUMM")
	// if exists {
	//	fmt.Fprintf(fd, globalTplcheckSumm, sTime, verify, ftPath)
	//} else {
	fmt.Fprintf(fd, globalTpl, sTime, ftPath)
	//}

	for _, rw := range cfg.OperationType {
		for _, bs := range cfg.BlockSize {
			var count = 0
			for _, depth := range cfg.DepthType {
				for _, job := range cfg.JobType {
					var section = fmt.Sprintf("%s-%s-%d", rw, bs, count)
					fmt.Fprintf(fd, sectionTpl, section, rw, bs, depth, job)
					count++
				}
			}
		}
	}

	return nil
}

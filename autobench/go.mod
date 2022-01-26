module github.com/zededa-yuri/nextgen-storage/autobench

go 1.16

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/jessevdk/go-flags v1.5.0
	github.com/lf-edge/eden v0.2.1-0.20220126233414-44ae5707c598
	github.com/lf-edge/eve/api/go v0.0.0-20220125064314-55e8c30d0b76
	github.com/pkg/sftp v1.13.2
	github.com/prometheus/procfs v0.7.3
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0 // indirect
	github.com/spf13/viper v1.10.0
	github.com/xuri/excelize/v2 v2.5.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf
	gonum.org/v1/plot v0.10.0
)

replace github.com/lf-edge/eden => ../../eden

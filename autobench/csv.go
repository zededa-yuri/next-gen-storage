package main

import (
	"fmt"

	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fioconv"
)

type CSVCommand struct {
	//GFormat 		string	`short:"t" long:"output-format" description:"Set type data in file. Maybe normal,json or json" required:"true"`
	GFilePath 		string	`short:"f" long:"file" description:"Source file" required:"true"`
	GName			string	`short:"n" long:"name" description:"Output file name" required:"true"`
}

var csvCmd CSVCommand

func init() {
	parser.AddCommand(
		"csv",
		"Create CSV table from json file",
		"",
		&csvCmd,
	)
}

func (x CSVCommand) Execute(args []string) error {
	if err := fioconv.ConvertJSONtoCSV(csvCmd.GFilePath, csvCmd.GName); err != nil {
		return fmt.Errorf("generate CSVtable failed: %w", err)
	}
	return nil
}

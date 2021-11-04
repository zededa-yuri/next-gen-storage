package main

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type PlotCommand struct {
	CResultLocation string `short:"f" long:"results" description:"The option takes the path to the PNG file"`
	CCatalog  		string `short:"c" long:"catalog" description:"Catalog with CSV results" required:"true"`
	CTestName 		string `short:"n" long:"title" description:"Title for test" default:"Autobench results"`
	CDescription	string `short:"d" long:"dsc" description:"Description for PNG image results"`
	CValue			string `short:"v" long:"value" description:"Performance measurement value" default:"Mb/s"`
	CType			string `short:"t" long:"type" description:"Graph rendering option (all, pattern) " default:"pattern"`
}

var plotCmd PlotCommand

// Curent format CSV: "Group ID", "Pattern", "Block Size", "IO Depth", "Jobs", "Mb/s"
type groupResults struct {
	groupID     string
	pattern     string
	bs          string
	depth       string
	jobsCount   string
	performance string
}

type testResult struct {
	groupRes groupResults
	pattern  string
}

type GroupRes []*testResult

type listAllResults struct {
	ioTestResults GroupRes
	fileName      string
	testName      string
}

type AllRes []*listAllResults


func (t *AllRes) parsingCSVfile(dir, fileName string) error {
	filePath := filepath.Join(dir, fileName)
	var groupFile = make(GroupRes, 0)
	csvFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open .csv file:[%s] failed! err:%v", filePath, err)
	}
	defer csvFile.Close()

	reader, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		return fmt.Errorf("parsing .csv file:[%s] failed! err:%v", filePath, err)
	}

	for iter, line := range reader {
		if iter == 0 {
			continue
		}
		resultOneGroup := groupResults{
			groupID:     line[0],
			pattern:     line[1],
			bs:          line[2],
			depth:       line[3],
			jobsCount:   line[4],
			performance: line[5],
		}
		group := testResult{
			groupRes: resultOneGroup,
			pattern: fmt.Sprintf("%s-%s-%s-%s",
				resultOneGroup.pattern,
				resultOneGroup.bs,
				resultOneGroup.depth,
				resultOneGroup.jobsCount),
		}
		groupFile = append(groupFile, &group)
	}
	finishRes := listAllResults{
		ioTestResults: groupFile,
		fileName:      filePath,
		testName:      fileName,
	}
	*t = append(*t, &finishRes)
	return nil
}

func plotCreate(testName, typeVolume, description string, xMax float64) (*plot.Plot, error) {
	p := plot.New()
	p.Title.Text = testName
	p.Y.Label.Text = typeVolume
	p.Y.Label.Padding = 10
	p.X.Label.Text = description
	p.X.Label.Position = 0
	p.X.Label.Padding = 25
	p.Legend.Top = true
	p.Legend.YOffs = vg.Length(+25)
	p.Legend.XOffs = vg.Length(-20)
	p.Legend.Padding = 2
	p.X.Max = xMax
	p.Title.Padding = 40
	p.X.Tick.Width = 0
	p.X.Tick.Length = 0
	p.X.Width = 0
	return p, nil
}

func drawBarChartPerformanceGroups(testName,
									typeVolume,
									description string,
									identicalPattern []string,
									results AllRes) error {

	p, _ := plotCreate(testName, typeVolume, description, float64(len(identicalPattern)))
	w := vg.Points(10)
	start := 0 - w

	for d, ipattern := range identicalPattern {
		for index, test := range results {
			for _, pattern := range test.ioTestResults {
				if pattern.pattern == ipattern {
					value, _ := strconv.ParseFloat(pattern.groupRes.performance, 64)
					bars, err := plotter.NewBarChart(plotter.Values{value}, w)
					if err != nil {
						return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
							identicalPattern, err)
					}
					bars.LineStyle.Width = vg.Length(0.3)
					bars.LineStyle.DashOffs = font.Length(0)
					bars.Color = plotutil.Color(index)
					bars.Width = 5
					start = start + w
					bars.Offset = start
					p.Add(bars)
					if d == 0 {
						p.Legend.Add(test.testName, bars)
					}

				}
			}
		}
		start = start + vg.Points(60)
	}

	p.Y.Padding = p.X.Tick.Label.Width(identicalPattern[0])
	p.X.Tick.Label.Rotation = -125
	ticks := make([]plot.Tick, len(identicalPattern))

	for i, name := range identicalPattern {
		ticks[i] = plot.Tick{float64(i), name}

	}
	p.X.Tick.Marker = plot.ConstantTicks(ticks)

	if err := p.Save(42*vg.Inch,
		8*vg.Inch,
		fmt.Sprintf("%s.png", testName)); err != nil {
		return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
			identicalPattern, err)
	}
	return nil
}

// DrawBarCharts generate bar charts for only one  groups between different tests
func DrawBarChartPerformanceOneGroup(testName, typeBar,
									 description,
									 identicalPattern,
									 dirResults string,
									 results AllRes) error {
	p, _ := plotCreate(testName, typeBar, description, 2)
	w := vg.Points(20)
	start := 0 - w
	p.NominalX(identicalPattern)

	for index, test := range results {
		for _, pattern := range test.ioTestResults {
			if pattern.pattern == identicalPattern {
				value, _ := strconv.ParseFloat(pattern.groupRes.performance, 64)
				bars, err := plotter.NewBarChart(plotter.Values{value}, w)
				if err != nil {
					return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
									  identicalPattern, err)
				}
				bars.LineStyle.Width = vg.Length(0)
				bars.Color = plotutil.Color(index)
				start = start + w
				bars.Offset = start
				p.Add(bars)
				p.Legend.Add(test.testName, bars)
			}
		}
	}

	if err := p.Save(6*vg.Inch,
		4*vg.Inch,
		fmt.Sprintf("%s.png", identicalPattern)); err != nil {
		return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
			identicalPattern, err)
	}
	return nil
}

func readDirWithResults(dirPath string) ([]fs.FileInfo, error) {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	return files, nil
}

//getIdenticalPatterns - search for identical results patterns
func getIdenticalPatterns(groups AllRes) []string {
	var allPattern []string
	uniq := make(map[string]int)

	for _, group := range groups {
		for _, result := range group.ioTestResults {
			uniq[result.pattern]++
		}
	}

	for key, val := range uniq {
		if val == len(groups) {
			allPattern = append(allPattern, key)
		}
	}

	return allPattern
}

func printRes(res AllRes) error {
	for i, test := range res {
		fmt.Println("Array ", i)
		fmt.Println(test.fileName, "| count pattern =",
				    len(test.ioTestResults), "iter:", i)
		for _, pattern := range test.ioTestResults {
			fmt.Println(pattern.pattern)
		}
		fmt.Println()
	}
	return nil
}

func (x *PlotCommand) Execute(args []string) error {
	dirRes := plotCmd.CResultLocation
	if dirRes == "" {
		chartsDirRes := filepath.Join(getSelfPath(), "results-charts")
		err := os.MkdirAll(chartsDirRes, os.ModeDir)
		if err != nil {
			fmt.Println("cannot create results-charts catalog:", err)
		}
	}

	var testResults = make(AllRes, 0)
	fileWithResults, err := readDirWithResults(plotCmd.CCatalog)
	if err != nil {
		return fmt.Errorf("file  %v", err)
	}

	for _, file := range fileWithResults {
		if !file.IsDir() {
			testResults.parsingCSVfile(plotCmd.CCatalog, file.Name())
		}
	}

	identicalPatterns := getIdenticalPatterns(testResults)

	if plotCmd.CType == "all" {
		if err := drawBarChartPerformanceGroups(plotCmd.CTestName,
		plotCmd.CValue,
		plotCmd.CDescription,
		identicalPatterns,
		testResults); err != nil {
			return fmt.Errorf("generate graphs failed%v", err)
		}
	} else {
		for _, pattern := range identicalPatterns {
			if err := DrawBarChartPerformanceOneGroup(plotCmd.CTestName,
											plotCmd.CValue,
											plotCmd.CDescription,
											pattern,
											dirRes,
											testResults); err != nil {
				return fmt.Errorf("generate graphs failed%v", err)
			}
		}
	}
	return nil
}

func init() {
	parser.AddCommand(
		"plot",
		"Plot graphics from JSON fio results",
		"This command creates graphics from JSON fio results",
		&plotCmd,
	)
}

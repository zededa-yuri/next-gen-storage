package main

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type PlotCommand struct {
	CCatalog  		string `short:"c" long:"catalog" description:"Catalog with CSV results" required:"true"`
	CDescription	string `short:"d" long:"dsc" description:"Description for PNG image results" default:"https://zededa.com"`
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
	bwMin       int
	bwMax       int
	IopsMin     int
	IopsMax     int
	latMin      float64
	latMax      float64
	latStd      float64
	cLatPercent float64
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

type allPatternResults struct {
	patternName  string
	values       []float64
	legends      []string
	yDiscription string
	fileName     string
}

type allLegendResults struct {
	legend       string
	value        []float64
	pattern      []string
	yDiscription string
	fileName     string
}

type legensTable []*allLegendResults
type patternsTable []*allPatternResults
type AllRes []*listAllResults

const (
	Performance uint16 = 1 << iota
	minIOPS
	maxIOPS
	minBW
	maxBW
	minLat
	maxLat
	stdLat
	p99Lat
)


func (t *AllRes) parsingCSVfile(dir, fileName string) error {
	filePath := filepath.Join(dir, fileName)
	var groupFile = make(GroupRes, 0)
	csvFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
	}
	defer csvFile.Close()

	reader, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		fmt.Println(err)
	}

	for iter, line := range reader {
		if iter == 0 {
			continue
		}
		bwmin, _ := strconv.Atoi(line[6])
		bwmax, _ := strconv.Atoi(line[7])
		pIopsMin, _ := strconv.Atoi(line[8])
		pIopsMax, _ := strconv.Atoi(line[9])
		pLatMin, _ := strconv.ParseFloat(line[10], 64)
		pLatMax, _ := strconv.ParseFloat(line[11], 64)
		pLatStd, _ := strconv.ParseFloat(line[12], 64)
		pLatP, _ := strconv.ParseFloat(line[13], 64)
		resultOneGroup := groupResults{
			groupID:     line[0],
			pattern:     line[1],
			bs:          line[2],
			depth:       line[3],
			jobsCount:   line[4],
			performance: line[5],
			bwMin:       bwmin,
			bwMax:       bwmax,
			IopsMin:     pIopsMin,
			IopsMax:     pIopsMax,
			latMin:      pLatMin,
			latMax:      pLatMax,
			latStd:      pLatStd,
			cLatPercent: pLatP,
		}
		group := testResult{
			groupRes: resultOneGroup,
			pattern: fmt.Sprintf("%s-%s d=%s j=%s",
				resultOneGroup.pattern,
				resultOneGroup.bs,
				resultOneGroup.depth,
				resultOneGroup.jobsCount),
		}
		groupFile = append(groupFile, &group)
	}
	testN := strings.Split(fileName, ".")
	finishRes := listAllResults{
		ioTestResults: groupFile,
		fileName:      filePath,
		testName:      testN[0],
	}
	*t = append(*t, &finishRes)
	return nil
}

//plotCreate - сreates a skeleton for plotting graphs
func plotCreate(testName, typeVolume, description string, xMax float64) (*plot.Plot, error) {
	p := plot.New()
	p.Title.Text = testName
	p.Title.TextStyle.Font.Size = font.Length(20)
	p.Y.Label.Text = typeVolume
	p.Y.Label.Padding = 10
	p.X.Label.Text = description
	p.X.Label.TextStyle.Font.Size = font.Length(10)
	p.X.Label.Padding = 25
	p.Legend.Top = true
	p.Legend.YOffs = vg.Length(+40)
	p.Legend.XOffs = vg.Length(-20)
	p.Legend.Padding = 2
	p.X.Max = xMax
	p.Title.Padding = 40
	p.X.Tick.Width = 0
	p.X.Tick.Length = 0
	p.X.Width = 0
	p.Add(plotter.NewGrid())
	return p, nil
}

func countingSizeCanvas(countBar int) (font.Length, font.Length) {
	if countBar > 22 {
		return 30 * vg.Inch, 7 * vg.Inch
	} else if countBar > 60 {
		return 55 * vg.Inch, 12 * vg.Inch
	}

	return 10 * vg.Inch, 7 * vg.Inch
}

func readDirWithResults(dirPath string) ([]fs.FileInfo, error) {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	return files, nil
}

//getIdenticalPatterns - search for identical results patterns
func getIdenticalPatterns(groups AllRes) ([]string, error) {
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

	if len(allPattern) == 0 {
		return nil, fmt.Errorf("error: the number of files is more than necessary")
	}
	return allPattern, nil
}

//getLegendsTable - Gets structures based on legends from patterns
func (t *legensTable) getLegendsTable(patternTable patternsTable) {
	for _, ilegend := range patternTable[0].legends {
		fTable := allLegendResults{
			legend: ilegend,
		}
		*t = append(*t, &fTable)
	}

	for _, ilegend := range *t {
		for _, pattern := range patternTable {
			for g := 0; g < len(pattern.values); g++ {
				if ilegend.legend == pattern.legends[g] {
					ilegend.value = append(ilegend.value, pattern.values[g])
					ilegend.pattern = append(ilegend.pattern, pattern.patternName)
				}
			}
			ilegend.yDiscription = pattern.yDiscription
			ilegend.fileName = pattern.fileName
		}
	}
}

//getPatternTable - Gets patterns based structures
func (t *patternsTable) getPatternTable(identicalPattern []string, results AllRes, valueType uint16) {
	for _, ipattern := range identicalPattern {
		fTable := allPatternResults{
			patternName: ipattern,
		}
		*t = append(*t, &fTable)
	}

	for _, stroka := range *t {
		for _, test := range results {
			for _, pattern := range test.ioTestResults {
				if pattern.pattern == stroka.patternName {
					var value float64
					switch val := valueType; val {
					case Performance:
						value, _ = strconv.ParseFloat(pattern.groupRes.performance, 64)
						stroka.yDiscription = "Mb/s"
						stroka.fileName = "Performance"
					case minIOPS:
						value = float64(pattern.groupRes.IopsMin)
						stroka.yDiscription = "IOPS min"
						stroka.fileName = "IOPS_min_value"
					case maxIOPS:
						value = float64(pattern.groupRes.IopsMax)
						stroka.yDiscription = "IOPS max"
						stroka.fileName = "IOPS_max_value"
					case minBW:
						value = float64(pattern.groupRes.bwMin)
						stroka.yDiscription = "BW Min (KiB/s)"
						stroka.fileName = "BW_min_value"
					case maxBW:
						value = float64(pattern.groupRes.bwMax)
						stroka.yDiscription = "BW Max (KiB/s)"
						stroka.fileName = "BW_max_value"
					case minLat:
						value = float64(pattern.groupRes.latMin)
						stroka.yDiscription = "Latency min (ms)"
						stroka.fileName = "Latency_min_value"
					case maxLat:
						value = float64(pattern.groupRes.latMax)
						stroka.yDiscription = "Latency max (ms)"
						stroka.fileName = "Latency_max_value"
					case stdLat:
						value = float64(pattern.groupRes.latStd)
						stroka.yDiscription = "Latency stddev (ms)"
						stroka.fileName = "Latency_stdev"
					case p99Lat:
						value = float64(pattern.groupRes.cLatPercent)
						stroka.yDiscription = "cLatency p99 (ms)"
						stroka.fileName = "Latency_p99"
					default:
						fmt.Println("Error with options")
					}
					stroka.values = append(stroka.values, value)
					stroka.legends = append(stroka.legends, test.testName)
				}
			}
		}
	}
}

//createBarCharts - generate bar charts for all groups between different tests
func (t *patternsTable) createBarCharts(table patternsTable, description, dirPath string) error {

	var lTable = make(legensTable, 0)
	lTable.getLegendsTable(table)
	p, _ := plotCreate(lTable[0].fileName, lTable[0].yDiscription, description, float64(len(table)))
	w := vg.Points(7)
	start := 0 - w
	for k := 0; k < len(lTable); k++ {
		var data plotter.Values
		data = lTable[k].value
		bars, _ := plotter.NewBarChart(data, w)
		bars.LineStyle.Width = vg.Length(0)
		bars.Width = font.Length(4)
		bars.Color = plotutil.Color(k)

		start = start + w
		bars.Offset = start
		p.Add(bars)
		p.Legend.Add(lTable[k].legend, bars)
	}

	p.X.Tick.Label.Rotation = -125
	ticks := make([]plot.Tick, len(*t))
	for i, name := range table {
		ticks[i] = plot.Tick{float64(i), name.patternName}
	}
	p.X.Tick.Width = font.Length(12)
	p.X.Tick.Marker = plot.ConstantTicks(ticks)
	p.X.Tick.Label.XAlign = -0.8

	width, height := countingSizeCanvas(len(table))

	if err := p.Save(width, height, filepath.Join(dirPath, fmt.Sprintf("%s.png", lTable[0].fileName))); err != nil {
		return fmt.Errorf("generate BarCharts for failed! err:%v", err)
	}
	return nil
}

//createBarChart - generate bar charts for only one groups between different tests
func (t *patternsTable) createBarChart(table patternsTable, description, dirPath string) error {
	resultsAbsDir := filepath.Join(dirPath, table[0].fileName)
	err := os.Mkdir(resultsAbsDir, 0755)
	if err != nil {
		fmt.Println("could not create local dir for result: %w", err)
	}

	for _, pattern := range table {
		p, _ := plotCreate(pattern.fileName, pattern.yDiscription, description, float64(len(table)))
		w := vg.Points(7)
		p.NominalX(pattern.patternName)
		start := 0 - w
		for i := 0; i < len(pattern.values); i++ {
			bars, err := plotter.NewBarChart(plotter.Values{pattern.values[i]}, w)
			if err != nil {
				return fmt.Errorf("generate BarCharts for [%s] failed! err:%v", pattern.patternName, err)
			}
			bars.LineStyle.Width = vg.Length(0)
			bars.Color = plotutil.Color(i)
			bars.Width = font.Length(4)
			start = start + w
			bars.Offset = start
			p.Add(bars)
			p.Legend.Add(pattern.legends[i], bars)
		}
		if err := p.Save(4*vg.Inch, 7*vg.Inch,
			filepath.Join(resultsAbsDir, fmt.Sprintf("%s.png", pattern.patternName))); err != nil {
			return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
				pattern.patternName, err)
		}
	}
	return nil
}

func InitBarCharts(dirWithCSV, descriptionForGraphs string) error {
	var testResults = make(AllRes, 0)

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}

	mainResultsAbsDir := filepath.Join(filepath.Dir(ex), "BarCharts")
	err = os.Mkdir(mainResultsAbsDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	fileWithResults, err := readDirWithResults(dirWithCSV)
	if err != nil {
		return fmt.Errorf("could not read dir with CSV files: %w", err)
	}

	for _, file := range fileWithResults {
		if !file.IsDir() {
			testResults.parsingCSVfile(dirWithCSV, file.Name())
		}
	}

	identicalPatterns, err := getIdenticalPatterns(testResults)
	if err != nil {
		return fmt.Errorf("could not get identical patterns: %w", err)
	}

	for _, valRes := range []uint16{Performance, minIOPS, maxIOPS, minBW, maxBW, minLat, maxLat, stdLat, p99Lat} {
		var pTable = make(patternsTable, 0)
		pTable.getPatternTable(identicalPatterns, testResults, valRes)
		pTable.createBarChart(pTable, descriptionForGraphs, mainResultsAbsDir)
		pTable.createBarCharts(pTable, descriptionForGraphs, mainResultsAbsDir)
	}

	return nil
}

func (x *PlotCommand) Execute(args []string) error {
	if err := InitBarCharts(plotCmd.CCatalog, plotCmd.CDescription); err != nil {
		return fmt.Errorf("error with create bar charts: %w", err)
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

package main

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math"
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
	CCatalog  		string	`short:"c" long:"catalog" description:"Catalog with CSV or Log results" required:"true"`
	CBarCharts		bool 	`short:"b" long:"barcharts" description:"Generate Bar Charts from CSV files"`
	CLogCharts		bool 	`short:"l" long:"logcharts" description:"Generate charts from log files"`
	CDescription	string	`short:"d" long:"dsc" description:"Description for PNG image results" default:"https://zededa.com"`
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

// Logs have line from fio bw/IOPS/latency log file.  (Example: 15, 204800, 0, 0);
// For bw log value == KiB/sec;
// For IOPS log value == count Iops;
// For Latency log value == latency in nsecs.
type LogLine struct {
	time     int // msec
	value    int
	opType   int // read - 0 ; write - 1 ; trim - 2
}

type LogFile []*LogLine

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

func toFixed(x float64, n int) float64 {
	var l = math.Pow(10, float64(n))
	var mbs = math.Round(x*l) / l
	return mbs * 1.049 // formula from google
}

func mbps(x int) float64 {
	return toFixed(float64(x)/1024, 2)
}

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

	if err := p.Save(width, height, filepath.Join(dirPath, fmt.Sprintf("%s.svg", lTable[0].fileName))); err != nil {
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
			filepath.Join(resultsAbsDir, fmt.Sprintf("%s.svg", pattern.patternName))); err != nil {
			return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
				pattern.patternName, err)
		}
	}
	return nil
}

func initBarCharts(dirWithCSV, descriptionForGraphs string) error {
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

func (t *LogFile) parsingLogfile(filePath string) error {
	logFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open log file %s err:%w", filePath, err)
	}
	defer logFile.Close()

	reader, err := csv.NewReader(logFile).ReadAll()
	if err != nil {
		return fmt.Errorf("could not read log file %s err:%w", filePath, err)
	}

	for _, line := range reader {
		// example line [119995  3481600  0  0]
		sTime, _ := strconv.Atoi(strings.TrimSpace(line[0]))
		sValue, _ := strconv.Atoi(strings.TrimSpace(line[1]))
		sOpType, _ := strconv.Atoi(strings.TrimSpace(line[2]))
		logLine := LogLine{
			time:   sTime,
			value:  sValue,
			opType: sOpType,
		}
		*t = append(*t, &logLine)
	}

	return nil
}

// getPoints - Get values for the x-axis
func getPoints(data LogFile, logType int) plotter.XYs {
	pts := make(plotter.XYs, len(data))
	for i, point := range data {
		pts[i].X = float64(i)
		if logType == 1 {
			pts[i].Y = mbps(point.value)
		} else {
			pts[i].Y = float64(point.value)
		}
	}
	return pts
}

//getInfoAboutLogFile   return: fileType (1-bw,2-iops,3-lat), Y-name, test Name, error
func getInfoAboutLogFile(fileName string) (int, string, string, error) {
	testName := strings.Split(fileName, ".")
	var err error

	bwLog := strings.Contains(fileName, "bw")
	if (bwLog) {
		return 1, "MB/s", testName[0], nil
	}

	iopsLog := strings.Contains(fileName, "iops")
	if (iopsLog) {
		return 2, "IOPS", testName[0], nil
	}

	latLog := strings.Contains(fileName, "lat")
	if (latLog) {
		return 3, "msec", testName[0], nil
	}

	return 0, "", "", err
}

// addLinePoints adds Line and Scatter plotters to a
// plot.  The variadic arguments must be either strings
// or plotter.XYers.  Each plotter.XYer is added to
// the plot using the next color, dashes, and glyph
// shape via the Color, Dashes, and Shape functions.
// If a plotter.XYer is immediately preceeded by
// a string then a legend entry is added to the plot
// using the string as the name.
//
// If an error occurs then none of the plotters are added
// to the plot, and the error is returned.
func addLinePoints(plt *plot.Plot, vs ...interface{}) error {
	var ps []plot.Plotter
	type item struct {
		name  string
		value [2]plot.Thumbnailer
	}
	var items []item
	name := ""
	var i int
	for _, v := range vs {
		switch t := v.(type) {
		case string:
			name = t

		case plotter.XYer:
			l, s, err := plotter.NewLinePoints(t)
			if err != nil {
				return err
			}
			l.Color = plotutil.Color(2)
			l.Dashes = plotutil.Dashes(0)
			l.StepStyle = plotter.NoStep
			s.Color = plotutil.Color(10)
			s.Shape = nil
			i++
			ps = append(ps, l, s)
			if name != "" {
				items = append(items, item{name: name, value: [2]plot.Thumbnailer{l, s}})
				name = ""
			}

		default:
			return fmt.Errorf("plotutil: AddLinePoints handles strings and plotter.XYers, got %T", t)
		}
	}
	plt.Add(ps...)
	for _, item := range items {
		v := item.value[:]
		plt.Legend.Add(item.name, v[0], v[1])
	}
	return nil
}

func countingSizeLogCanvas(countX int) (font.Length, font.Length) {
	if countX > 30000 {
		return 400 * vg.Inch, 10 * vg.Inch
	} else if countX > 20000 {
		return 250 * vg.Inch, 7 * vg.Inch
	} else if countX > 10000 {
		return 150 * vg.Inch, 7 * vg.Inch
	} else if countX > 5000 {
		return 100 * vg.Inch, 7 * vg.Inch
	} else if countX > 1000 {
		return 70 * vg.Inch, 7 * vg.Inch
	} else if countX > 500 {
		return 35 * vg.Inch, 7 * vg.Inch
	} else if countX > 100 {
		return 20 * vg.Inch, 7 * vg.Inch
	}
	return 10 * vg.Inch, 7 * vg.Inch
}

func (t *LogFile) createLogChart(data LogFile, logType int, yName, testName, discription, DirResPath string) error {
	p, _ := plotCreate(testName, yName, "description", float64(len(data)))
	err := addLinePoints(p, yName, getPoints(data, logType))
	if err != nil {
		return fmt.Errorf("error with get values for the Y-axis %w", err)
	}

	width, height := countingSizeLogCanvas(len(data))
	if err := p.Save(width, height, filepath.Join(DirResPath, fmt.Sprintf("%s.svg", testName)) ); err != nil {
		return fmt.Errorf("error with save charts %w", err)
	}
	return nil
}

func initLogCharts(dirWithLogs, discription string) error {
	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}

	mainResultsAbsDir := filepath.Join(filepath.Dir(ex), "logCharts")
	err = os.Mkdir(mainResultsAbsDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	fileWithResults, err := readDirWithResults(dirWithLogs)
	if err != nil {
		return fmt.Errorf("could not read dir with CSV files: %w", err)
	}

	for _, fileName := range fileWithResults {
		if !fileName.IsDir() {
 			var logData = make(LogFile, 0)
			fType, yName, testName, _  := getInfoAboutLogFile(fileName.Name())
			logData.parsingLogfile(filepath.Join(dirWithLogs, fileName.Name()))
			logData.createLogChart(logData, fType, yName, testName, discription, mainResultsAbsDir)
		}
	}
	return nil
}

func (x *PlotCommand) Execute(args []string) error {
	if plotCmd.CBarCharts {
		if err := initBarCharts(plotCmd.CCatalog, plotCmd.CDescription); err != nil {
			return fmt.Errorf("error with create bar charts: %w", err)
		}
	} else if plotCmd.CLogCharts {
		if err := initLogCharts(plotCmd.CCatalog, plotCmd.CDescription); err != nil {
			return fmt.Errorf("error with create log charts: %w", err)
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

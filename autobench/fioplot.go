package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
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
	"github.com/xuri/excelize/v2"
)

type PlotCommand struct {
	CCatalog  		string	`short:"c" long:"catalog" description:"Catalog with CSV or Log results" required:"true"`
	CBarCharts		bool 	`short:"b" long:"barcharts" description:"Generate Bar Charts from CSV files"`
	CLogCharts		bool 	`short:"l" long:"logcharts" description:"Generate charts from log files"`
	CDescription	string	`short:"d" long:"dsc" description:"Description for PNG image results" default:"https://zededa.com"`
	CXcelCharts		bool	`short:"e" long:"excel" description:"Save results to a shared excel table"`
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
type GroupLogFiles struct {
	filesPath 		[]string
	patternName 	string
	minCountLine 	int
}

type LogGF []*GroupLogFiles
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

const barTpl = `
	{
		"name": "%s",
		"categories": "%s!%s",
		"values": "%s!%s"
	}%s`

const globalTpl = `{
	"type": "col",
	"series":
	%s
	,
	"y_axis":
	{
		"major_grid_lines": true,
		"minor_grid_lines": true
	},
	"x_axis":
	{
		"major_grid_lines": true
	},
	"legend":
	{
		"position": "left",
		"show_legend_key": true
	},
	"title":
	{
		"name": "%s"
	}
}`

func toFixed(x float64, n int) float64 {
	var l = math.Pow(10, float64(n))
	var mbs = math.Round(x*l) / l
	return mbs * 1.049 // formula from google
}

func mbps(x int) float64 {
	return toFixed(float64(x)/1024, 2)
}

// Round performance value
func Round(x float64) float64 {
	t := math.Trunc(x)
	if math.Abs(x-t) >= 0.5 {
		return t + math.Copysign(1, x)
	}
	return t
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
	} else if countBar > 100 {
		return 110 * vg.Inch, 12 * vg.Inch
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
	fmt.Println("Read files:")
	for _, group := range groups {
		/* This output of a list of files is necessary to
		* understand which files we are transferring for
		* comparison. Sometimes it happens that there
		*are hidden temporary files in the folder! */
		fmt.Println(group.fileName)
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
						tmpBw, _ := strconv.ParseFloat(pattern.groupRes.performance, 64)
						value = Round(tmpBw)
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
	w := vg.Points(3)
	start := 0 - w
	for k := 0; k < len(lTable); k++ {
		var data plotter.Values
		data = lTable[k].value
		bars, _ := plotter.NewBarChart(data, w)
		bars.LineStyle.Width = vg.Length(0)
		bars.Width = font.Length(2)
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
	p.X.Tick.Width = font.Length(8)
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

//createBarCharts - generate table for all groups between different tests in Excel
func (t *patternsTable) createExcelTables(table patternsTable, description, filePath string) error {

 	f, err := excelize.OpenFile(filePath)
	if err != nil {
    	return fmt.Errorf("open %s xlsx file failed: %w", filePath, err)
	}

	rowIter := 2
	sheetName := ""
	for _, pattern := range table {
		if sheetName != pattern.fileName {
			sheetName = pattern.fileName
			var headerXSLS = []string{"Pattern"}
			f.NewSheet(sheetName)
			f.SetColWidth(sheetName, "A", "A", 25)
			f.SetSheetRow(sheetName, "A1", &headerXSLS)
			f.SetSheetRow(sheetName, "B1", &pattern.legends)
			rowIter = 2
		}
		var patternName = []string{pattern.patternName}
		f.SetSheetRow(sheetName, fmt.Sprintf("A%d", rowIter), &patternName)
		f.SetSheetRow(sheetName, fmt.Sprintf("B%d", rowIter), &pattern.values)
		rowIter++
	}

	f.DeleteSheet("Sheet1") //remove default sheet
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("could save xlsx file failed %w", err)
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

func (t *LogFile) SaveFile(filePath string, countLine int) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error create file [%s] %w", filePath, err)
	}
	defer file.Close()

	for index, line := range *t {
		if index == countLine {
			break
		}
		file.WriteString(fmt.Sprintf("%d, %d, %d, 0\n", line.time, line.value, line.opType))
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

func lineCounter(r io.Reader) (int, error) {
    buf := make([]byte, 32*1024)
    count := 0
    lineSep := []byte{'\n'}

    for {
        c, err := r.Read(buf)
        count += bytes.Count(buf[:c], lineSep)

        switch {
        case err == io.EOF:
            return count, nil

        case err != nil:
            return count, err
        }
    }
}

func getCountLineInFile(path string) int {
	file, _ := os.Open(path)
	countLine, err := lineCounter(file)
	if err != nil {
		fmt.Println("counting line in file failed! error:%w", path, err)
		return 0
	}
	return countLine
}

func (t *LogGF) SepareteLogs(fileList []fs.FileInfo, sessonPath string) error {
	haveName := false
	for _, file := range fileList {
		fTable := GroupLogFiles{}
		countLine := getCountLineInFile(fmt.Sprintf("%s/%s", sessonPath, file.Name()))
		fullPathToFile := fmt.Sprintf("%s/%s", sessonPath, file.Name())
		patternFile := strings.Split(file.Name(), ".")[0]
		haveName = false
		for _, value := range *t {
			if value.patternName == patternFile {
				value.filesPath = append(value.filesPath, fullPathToFile)
				if value.minCountLine > countLine {
					value.minCountLine = countLine
				}
				haveName = true
			}
		}
		if !haveName {
			fTable.patternName = patternFile
			fTable.filesPath = append(fTable.filesPath, fullPathToFile)
			fTable.minCountLine = countLine
			*t = append(*t, &fTable)
		}
	}
	return nil
}

func (t *LogGF) GluingFiles(resultsDir string) error {
	for _, value := range *t {
		var logDataMainFile = make(LogFile, 0)
		logDataMainFile.parsingLogfile(value.filesPath[0])
		if len(value.filesPath) == 1 {
			// There is nothing to glue here, just saving the file
			err := logDataMainFile.SaveFile(filepath.Join(resultsDir, fmt.Sprintf("%s.log", value.patternName)), value.minCountLine)
			if err != nil {
				return fmt.Errorf("error create file %w", err)
			}
			continue

		}
		for index := 1; index < len(value.filesPath); index++ {
			var logTmpFile = make(LogFile, 0)
			logTmpFile.parsingLogfile(value.filesPath[index])

			for line := 0; line < value.minCountLine; line++ {
				logDataMainFile[line].value += logTmpFile[line].value
			}
		}

		err := logDataMainFile.SaveFile(filepath.Join(resultsDir, fmt.Sprintf("%s.log", value.patternName)), value.minCountLine)
		if err != nil {
			return fmt.Errorf("error create file %w", err)
		}
	}
	return nil
}

func GetLogFilesFromGroup(dirWithLogs, mainResultsAbsDir string, fileList []fs.FileInfo) error {

	var logGroupF = make(LogGF, 0)
	logGroupF.SepareteLogs(fileList, dirWithLogs)
	logGroupF.GluingFiles(mainResultsAbsDir)

	return nil
}

func initLogCharts(dirWithLogs, discription string) error {
	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}

	mainResultsAbsDirCharts := filepath.Join(filepath.Dir(ex), "logCharts")
	err = os.Mkdir(mainResultsAbsDirCharts, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	mainLogsAbsDir := filepath.Join(filepath.Dir(ex), "GluedLogFiles")
	err = os.Mkdir(mainLogsAbsDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir %s for result: %w", mainLogsAbsDir, err)
	}

	fileWithResultsSrc, err := readDirWithResults(dirWithLogs)
	if err != nil {
		return fmt.Errorf("could not read dir with log files: %w", err)
	}

	if err := GetLogFilesFromGroup(dirWithLogs, mainLogsAbsDir, fileWithResultsSrc); err != nil {
		return fmt.Errorf("could not glued log files: %w", err)
	}

	fileWithResults, err := readDirWithResults(mainLogsAbsDir)
	if err != nil {
		return fmt.Errorf("could not read dir with log files: %w", err)
	}

	for _, fileName := range fileWithResults {
		if !fileName.IsDir() {
 			var logData = make(LogFile, 0)
			fType, yName, testName, _  := getInfoAboutLogFile(fileName.Name())
			logData.parsingLogfile(filepath.Join(mainLogsAbsDir, fileName.Name()))
			logData.createLogChart(logData, fType, yName, testName, discription, mainResultsAbsDirCharts)
		}
	}
	return nil
}

func genExcelfile(pathFile string) bool {
	f := excelize.NewFile()
	if err := f.SaveAs(pathFile); err != nil {
		fmt.Println("could save xlsx file failed %w", err)
		return false
	}
	return true
}

func createExcelCharts(countStroke int, filePath string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
    	return fmt.Errorf("open %s xlsx file failed: %w", filePath, err)
	}
	//letters with a margin
	str := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
	finishStr := []string{}
	sheets := f.GetSheetList()
	for index, sheet := range sheets {
		value, _ := f.GetCellValue(sheet, fmt.Sprintf("%s1", str[index]))
		if len(value) == 0 {
			break
		} else if value == "Pattern" {
			continue
		}
		finishStr = append(finishStr, str[index])
	}

	f.NewSheet("Bars")
	barsIndent := 1

	for _, sheet := range sheets {
		series := []string{}
		point := ","
		for i, char := range finishStr {
			n1, err := f.GetCellValue(sheet, fmt.Sprintf("%s1", char))
			if err != nil {
				fmt.Println("kek")
			}
			if i == len(finishStr) -1 {
				point = ""
			}
			bar := fmt.Sprintf(barTpl, n1,
						sheet, fmt.Sprintf("$A$%d:$A$%d", 2, countStroke),
						sheet, fmt.Sprintf("$%s$%d:$%s$%d", char, 2, char, countStroke), point)
			series = append(series, bar)
		}

		chartsP := fmt.Sprintf(globalTpl, series, sheet)

		if err := f.AddChart("Bars", fmt.Sprintf("B%d", barsIndent), chartsP); err != nil {
			fmt.Println(err)
		}
		barsIndent += 20
	}

	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("could save xlsx file failed %w", err)
	}
	return nil
}

func initExcelCharts(dirWithCSV, descriptionForGraphs string) error {
	var testResults = make(AllRes, 0)
	countStroke := 0

	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}

	mainResultsFile := filepath.Join(filepath.Dir(ex), "all-results.xlsx")
	if !genExcelfile(mainResultsFile) {
		return fmt.Errorf("could not create excel file")
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
		pTable.createExcelTables(pTable, descriptionForGraphs, mainResultsFile)
		countStroke = len(pTable)
	}

	if err := createExcelCharts(countStroke + 1, mainResultsFile); err != nil {
		return fmt.Errorf("could not create excel charts: %w", err)
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
	} else if plotCmd.CXcelCharts {
		if err := initExcelCharts(plotCmd.CCatalog, plotCmd.CDescription); err != nil {
			return fmt.Errorf("error with create excel charts: %w", err)
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

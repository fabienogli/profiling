package main

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	dateOutFmt      = "15:04:05"
	profileFileName = "profile.csv"
)

var (
	errRecNotGood = errors.New("record not the good length")
	//go:embed chart.js index.html
	staticFS embed.FS
)

type Row struct {
	Date    time.Time
	CpuPct  float64
	Mem     float64
	Process string
}

type Table struct {
	Date    []time.Time
	CpuPct  []float64
	Mem     []float64
	Process []string
}

func main() {
	http.Handle("/", http.FileServer(http.FS(staticFS)))
	http.HandleFunc("/data", dataHandler)
	log.Println("Starting server on ", "http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func generateData() error {
	delimiter := "%CPU %MEM ARGS mer."
	rawProfile, err := os.ReadFile("./ps.log")
	if err != nil {
		return err
	}
	lines := [][]string{
		{"time", "cpu", "mem", "proc"},
	}
	test := strings.Split(string(rawProfile), delimiter)
	for _, s := range test {
		if s == "" {
			continue
		}
		t := strings.Split(s, "\n")
		dateStr := strings.TrimSpace(t[0])
		date, err := time.Parse("02 Jan. 2006 15:04:05 MST", dateStr)
		if err != nil {
			log.Printf("line ='%s'", s)
			log.Printf("dateStr ='%s'", dateStr)
			return err
		}
		for _, o := range t[1:] {
			o = strings.TrimSpace(o)
			a := strings.Fields(o)
			if len(a) < 3 {
				continue
			}
			profile := []string{
				date.Format(dateOutFmt),
				a[0],
				a[1],
				strings.Join(a[2:], " "),
			}
			lines = append(lines, profile)
		}
	}
	csvFile, err := os.Create(profileFileName)
	if err != nil {
		return err
	}
	defer csvFile.Close()
	csvwriter := csv.NewWriter(csvFile)
	err = csvwriter.WriteAll(lines)
	if err != nil {
		return err
	}
	return nil
}

func readData() (Table, error) {
	var table Table
	f, err := os.Open(profileFileName)
	if err != nil {
		return table, err
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	if err != nil {
		return table, err
	}
	_, err = csvReader.Read()
	if err != nil {
		log.Printf("Error while decoding header: %s \n%s", err)
		return table, err
	}
	for rec, err := []string{}, err; err == nil; rec, err = csvReader.Read() {
		if errors.Is(err, io.EOF) {
			return table, nil
		}
		row, err := decodeRow(rec)

		if err != nil {
			log.Printf("Error while decoding row: %s \n'%v'", rec, err)
			continue
			return table, err
		}
		table.Date = append(table.Date, row.Date)
		table.CpuPct = append(table.CpuPct, row.CpuPct)
		table.Mem = append(table.Mem, row.Mem)
		table.Process = append(table.Process, row.Process)
	}
	return table, nil
}

func decodeRow(rec []string) (Row, error) {
	if len(rec) < 4 {
		return Row{}, errRecNotGood
	}
	date, err := time.Parse(dateOutFmt, rec[0])
	if err != nil {
		return Row{}, err
	}
	cpu, err := strconv.ParseFloat(rec[1], 64)
	if err != nil {
		return Row{}, err
	}
	mem, err := strconv.ParseFloat(rec[2], 64)
	if err != nil {
		return Row{}, err
	}
	process := rec[3]
	return Row{
		Date:    date,
		CpuPct:  cpu,
		Mem:     mem,
		Process: process,
	}, nil
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Inside dataHandler")
	_ = r.URL.Query().Get("symbol")
	table, err := readData()
	if err != nil {
		log.Printf("can't fetch data: %s", err)
		http.Error(w, "can't fetch data", http.StatusInternalServerError)
		return
	}

	if err := tableJSON(table, w); err != nil {
		log.Printf("table: %s", err)
	}
}

func tableJSON(table Table, w io.Writer) error {
	min, err := time.Parse(dateOutFmt, "10:58:26")
	if err != nil {
		return err
	}
	max, err := time.Parse(dateOutFmt, "18:14:58")
	if err != nil {
		return err
	}
	reply := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"x":    table.Date,
				"y":    table.CpuPct,
				"name": "cpu",
				"mode": "line",
				"type": "scatter",
			},
		},
		"layout": map[string]interface{}{
			"title": "Cpu Usage over time",
			"xaxis": map[string]interface{}{
				"type":  "date",
				"title": "heure",
				"range": []time.Time{
					min,
					max,
				},
			},
			"yaxis": map[string]interface{}{
				"type":  "linear",
				"title": "cpu usage %",
				"color": "blue",
				"range": []int{
					0,
					500,
				},
			},
		},
	}
	return json.NewEncoder(w).Encode(reply)
}

//https://www.ardanlabs.com/blog/2022/01/visualizations-in-go.html

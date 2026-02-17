package screens

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type StationInfo struct {
	NumPost    string
	CommonName string
	Lat        float64
	Lon        float64
	Alti       float64
}

type WeatherData struct {
	StationInfo
	ObsDate time.Time
	Rain    float64
}

func ReadRRTVentFile(logger *slog.Logger, filename string) []WeatherData {
	f, err := os.Open(fmt.Sprintf("./data/%s", filename))
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	csvReader := csv.NewReader(f)
	csvReader.Comma = ';'

	data := make([]WeatherData, 0, 1000000)

	line := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("Error while reading line", "error", err, "line", line, "filename", filename)
		} else {
			lat, err := strconv.ParseFloat(record[2], 64)
			if err != nil {
				logger.Error("Error while parsing line", "error", err, "line", line, "value", record[2])
				continue
			}
			lon, err := strconv.ParseFloat(record[3], 64)
			if err != nil {
				logger.Error("Error while parsing line", "error", err, "line", line, "value", record[3])
				continue
			}
			alt, err := strconv.ParseFloat(record[4], 64)
			if err != nil {
				logger.Error("Error while parsing line", "error", err, "line", line, "value", record[4])
				continue
			}
			layout := "20060102"

			t, err := time.Parse(layout, record[5])
			if err != nil {
				logger.Error("Error while parsing line", "error", err, "line", line, "value", record[5])
				continue
			}
			rain, err := strconv.ParseFloat(record[6], 64)
			if err != nil {
				logger.Error("Error while parsing line", "error", err, "line", line, "value", record[6])
				continue
			}
			data = append(data, WeatherData{
				StationInfo: StationInfo{
					NumPost:    record[0],
					CommonName: record[1],
					Lat:        lat,
					Lon:        lon,
					Alti:       alt,
				},
				ObsDate: t,
				Rain:    rain,
			})
		}
		line++
	}

	return data
}

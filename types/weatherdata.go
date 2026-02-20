package types

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"os"
	"slices"
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

type WeatherRecord struct {
	StationInfo
	ObsDate time.Time
	Rain    float64
}

type WeatherData struct {
	Data     []WeatherRecord
	Stations []StationInfo
}

func (wd *WeatherData) ClosestStation(long, lat float64) *StationInfo {
	minD := math.MaxFloat64
	var closestStation StationInfo
	for _, station := range wd.Stations {
		dy := math.Abs(station.Lat - lat)
		dx := math.Abs(station.Lon - long)

		d := math.Sqrt(math.Pow(dy, 2) + math.Pow(dx, 2))

		if d < minD {
			minD = d
			closestStation = station
		}
	}

	return &closestStation
}

func NewWeatherData(logger *slog.Logger, department string) *WeatherData {
	records, err := ReadRRTVentFile(logger, fmt.Sprintf("Q_%s_previous-1950-2024_RR-T-Vent.csv", department))
	stations := make([]StationInfo, 0, 50)
	if err == nil {
		newStations := getStationList(records)
		stations = slices.Concat(stations, newStations)
	}

	freshRecord, err := ReadRRTVentFile(logger, fmt.Sprintf("Q_%s_latest-2025-2026_RR-T-Vent.csv", department))
	if err == nil {
		records = slices.Concat(records, freshRecord)
		newStations := getStationList(records)
		stations = slices.Concat(stations, newStations)
	}

	return &WeatherData{
		Data:     records,
		Stations: stations,
	}
}

func ReadRRTVentFile(logger *slog.Logger, filename string) ([]WeatherRecord, error) {
	f, err := os.Open(fmt.Sprintf("./data/%s", filename))
	if err != nil {
		return nil, err
	}

	defer f.Close()

	csvReader := csv.NewReader(f)
	csvReader.Comma = ';'

	data := make([]WeatherRecord, 0, 1000000)

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
			data = append(data, WeatherRecord{
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

	return data, nil
}

func getStationList(wd []WeatherRecord) []StationInfo {
	stationMap := make(map[string]StationInfo)
	for i := range wd {
		if _, exist := stationMap[wd[i].NumPost]; !exist {
			stationMap[wd[i].NumPost] = wd[i].StationInfo
		}
	}

	return slices.Collect(maps.Values(stationMap))
}

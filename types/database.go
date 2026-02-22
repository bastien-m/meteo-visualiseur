package types

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strconv"
	"time"

	// _ "modernc.org/sqlite"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type FileProcessedModel struct {
	gorm.Model
	Department string `gorm:"index;unique"`
}

type StationModel struct {
	NumPost    string `gorm:"primaryKey"`
	CommonName string
	Lat        float64
	Long       float64
	Alti       float64
}

type WeatherRecordModel struct {
	StationNumPost string `gorm:"primaryKey"`
	ObsDate        string `gorm:"primaryKey"`
	Rain           float64
}

func InitDB(dbPath string, debug bool) (*gorm.DB, error) {
	logMode := logger.Silent
	if debug {
		logMode = logger.Info
	}
	db, err := gorm.Open(sqlite.Open("meteo.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logMode),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	err = db.AutoMigrate(&FileProcessedModel{}, &StationModel{}, &WeatherRecordModel{})

	if err != nil {
		return nil, fmt.Errorf("Can't create models %w", err)
	}

	return db, nil
}

func ImportWeatherData(db *gorm.DB, logger *slog.Logger, department string) error {
	importFile := &FileProcessedModel{}
	result := db.Where("department = ?", department).First(importFile)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		var allRecords []WeatherRecordModel
		var allStations []StationModel

		records, stations, err := loadRRTVentFile(logger, fmt.Sprintf("Q_%s_previous-1950-2024_RR-T-Vent.csv", department))
		if err != nil {
			logger.Warn("No historical CSV found", "department", department, "error", err)
		} else {
			allRecords = slices.Concat(allRecords, records)
			allStations = slices.Concat(allStations, stations)
		}

		freshRecords, stations, err := loadRRTVentFile(logger, fmt.Sprintf("Q_%s_latest-2025-2026_RR-T-Vent.csv", department))
		if err != nil {
			logger.Warn("No recent CSV found", "department", department, "error", err)
		} else {
			allRecords = slices.Concat(allRecords, freshRecords)
			allStations = slices.Concat(allStations, stations)
		}

		if len(allRecords) == 0 {
			return fmt.Errorf("no CSV data found for department %s", department)
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			// Create Weather Records
			result := db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(allRecords, 10_000)

			logger.Info("Import complete", "department", department, "stations", result.RowsAffected)

			if result.Error != nil {
				return result.Error
			}

			// Create Station Records
			result = db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(allStations, 100)

			if result.Error != nil {
				return result.Error
			}

			result = db.Create(&FileProcessedModel{
				Department: department,
			})

			if result.Error != nil {
				return result.Error
			}

			return nil
		})

		return err
	} else if result.Error != nil {
		return result.Error
	}

	return nil

}

type RainByStation struct {
	NumPost string
	Year    string
	Rain    float64
}

func GetRainByStation(db *gorm.DB, numPost string) ([]RainByStation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var result []RainByStation
	err := db.WithContext(ctx).Model(&WeatherRecordModel{}).
		Select("station_num_post as num_post, substr(obs_date, 1, 4) as year, sum(rain) as rain").
		Where("station_num_post = ?", numPost).
		Group("station_num_post, year").
		Having("count(1) > 365 * 0.95").
		Find(&result).Error

	if err != nil {
		return nil, fmt.Errorf("Can't find station %s: %w", numPost, err)
	}

	return result, nil
}

func GetStationsForDepartment(db *gorm.DB, dpt string) []StationModel {
	var stations []StationModel
	db.Where("num_post LIKE ?", dpt+"%").Find(&stations)

	return stations
}

func GetClosestStation(db *gorm.DB, long, lat float64) (*StationModel, error) {
	var closestStation StationModel
	err := db.Raw("SELECT num_post, common_name, long, lat FROM station_models ORDER BY (lat - ?)*(lat - ?) + (long - ?)*(long - ?) LIMIT 1", lat, lat, long, long).
		Scan(&closestStation).Error

	if err != nil {
		return nil, err
	}

	return &closestStation, nil
}

func GetFilesImported(db *gorm.DB) (result []FileProcessedModel, err error) {
	err = db.Find(&result).Error
	return result, err
}

func loadRRTVentFile(logger *slog.Logger, filename string) (weatherRecords []WeatherRecordModel, stations []StationModel, err error) {
	// TODO: in future version we should stream data from data.gouv here
	f, err := os.Open(fmt.Sprintf("./data/%s", filename))
	if err != nil {
		return nil, nil, err
	}

	defer f.Close()

	csvReader := csv.NewReader(f)
	csvReader.Comma = ';'

	weatherRecords = make([]WeatherRecordModel, 0, 1000000)
	stationsMap := make(map[string]StationModel, 100)

	line := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("Error while reading line", "error", err, "line", line, "filename", filename)
		} else {
			layout := "20060102"

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
			weatherRecords = append(weatherRecords, WeatherRecordModel{
				StationNumPost: record[0],
				ObsDate:        t.Format("20060102"),
				Rain:           rain,
			})

			stationsMap[record[0]] = StationModel{
				NumPost:    record[0],
				CommonName: record[1],
				Lat:        lat,
				Long:       lon,
				Alti:       alt,
			}
		}
		line++
	}

	return weatherRecords, slices.Collect(maps.Values(stationsMap)), nil
}

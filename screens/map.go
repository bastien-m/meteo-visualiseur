package screens

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenWidth   = 600
	screenHeight  = 600
	neubourg      = "27428002"
	secondPerYear = 2.0
)

type ScreenMap struct {
	geojson                          FranceGeoJSON
	minLong, maxLong, minLat, maxLat float64
	startTime                        time.Time
	logger                           *slog.Logger
}

func (sm *ScreenMap) Update() error {
	return nil
}

func (sm *ScreenMap) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 255, 255})
	ebitenutil.DebugPrintAt(screen, "France", 10, 10)
	sm.displayDepartmentWeather(screen, "27")
	sm.drawFranceOutline(screen)
}

func (sm *ScreenMap) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func Run(logger *slog.Logger) {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Météo")

	geojson := readGeoJsonFile(logger)

	minLong, maxLong, minLat, maxLat := minMaxLongLat(geojson)
	screenMap := &ScreenMap{
		geojson:   geojson,
		minLong:   minLong,
		maxLong:   maxLong,
		minLat:    minLat,
		maxLat:    maxLat,
		startTime: time.Now(),
		logger:    logger,
	}

	if err := ebiten.RunGame(screenMap); err != nil {
		log.Fatal(err)
	}
}

type Coordinate []float64

type FranceGeoJSON struct {
	Geometry struct {
		Coordinates [][][]Coordinate `json:"coordinates"`
	} `json:"geometry"`
}

func (sm *ScreenMap) drawFranceOutline(screen *ebiten.Image) {
	var prevX, prevY float64

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%f", sm.minLong), 10, 60)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%f", sm.maxLong), 10, 90)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%f", sm.minLat), 10, 120)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%f", sm.maxLat), 10, 150)

	for i := range sm.geojson.Geometry.Coordinates {
		for j := range sm.geojson.Geometry.Coordinates[i] {
			for k := range sm.geojson.Geometry.Coordinates[i][j] {
				outline := sm.geojson.Geometry.Coordinates[i][j][k]

				if k == 0 {
					prevX, prevY = sm.getScreenPosition(outline[0], outline[1])
					continue
				}

				lineColor := color.RGBA{255, 255, 255, 255}

				x, y := sm.getScreenPosition(outline[0], outline[1])

				vector.StrokeLine(screen, float32(prevX), float32(prevY), float32(x), float32(y), 2.0, lineColor, true)

				prevX = x
				prevY = y
			}
		}
	}

	ebitenutil.DebugPrintAt(screen, "Finish drawing", 10, 30)

}

func readGeoJsonFile(logger *slog.Logger) FranceGeoJSON {
	filepath := "./data/metropole-version-simplifiee.geojson"
	file, err := os.ReadFile(filepath)
	if err != nil {
		logger.Error("Can't read file", "error", err, "filepath", filepath)
	}

	geojson := FranceGeoJSON{}
	err = json.Unmarshal(file, &geojson)
	if err != nil {
		logger.Error("Can't parse to json", "error", err)
	}
	return geojson
}

func minMaxLongLat(geoJson FranceGeoJSON) (float64, float64, float64, float64) {
	minLat := math.MaxFloat64
	maxLat := -math.MaxFloat64
	minLong := math.MaxFloat64
	maxLong := -math.MaxFloat64

	for i := range geoJson.Geometry.Coordinates {
		for j := range geoJson.Geometry.Coordinates[i] {
			for k := range geoJson.Geometry.Coordinates[i][j] {
				coordinate := geoJson.Geometry.Coordinates[i][j][k]

				currentLong := coordinate[0]
				currentLat := coordinate[1]

				if currentLong > maxLong {
					maxLong = currentLong
				} else if currentLong < minLong {
					minLong = currentLong
				}

				if currentLat > maxLat {
					maxLat = currentLat
				} else if currentLat < minLat {
					minLat = currentLat
				}
			}

		}

	}

	return minLong, maxLong, minLat, maxLat
}

func (sm *ScreenMap) getScreenPosition(long float64, lat float64) (float64, float64) {
	x := (long - sm.minLong) * screenWidth / (sm.maxLong - sm.minLong)
	y := screenHeight - (lat-sm.minLat)*screenHeight/(sm.maxLat-sm.minLat)

	return x, y
}

func (sm *ScreenMap) displayDepartmentWeather(screen *ebiten.Image, department string) {
	weather := ReadRRTVentFile(sm.logger, fmt.Sprintf("Q_%s_previous-1950-2024_RR-T-Vent.csv", department))

	min, max := getFirstLastObsDateForStation(neubourg, weather)

	deltaTime := time.Since(sm.startTime)

	currentYear := min.Year() + (int(deltaTime.Seconds()/secondPerYear) % (max.Year() - min.Year()))

	rainInYear := 0.0
	var textX, textY float64
	for _, post := range weather {
		if post.NumPost == neubourg {
			textX, textY = sm.getScreenPosition(post.Lon, post.Lat)
			if post.ObsDate.Year() == currentYear {
				rainInYear += post.Rain
			}
		}
	}

	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Rain in %d: %f", currentYear, rainInYear), int(math.Floor(textX)), int(math.Floor(textY)))

}

func getFirstLastObsDateForStation(station string, weatherData []WeatherData) (time.Time, time.Time) {
	min := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	max := time.Date(1949, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := range weatherData {
		if weatherData[i].NumPost == station {
			if weatherData[i].ObsDate.Before(min) {
				min = weatherData[i].ObsDate
			} else if weatherData[i].ObsDate.After(max) {
				max = weatherData[i].ObsDate
			}
		}
	}

	return min, max
}

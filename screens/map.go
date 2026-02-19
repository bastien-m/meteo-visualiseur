package screens

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"log/slog"
	"maps"
	"math"
	"meteo/components"
	"os"
	"slices"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Coordinate []float64

type FranceGeoJSON struct {
	Geometry struct {
		Coordinates [][][]Coordinate `json:"coordinates"`
	} `json:"geometry"`
}

const (
	screenWidth      = 600
	screenHeight     = 800
	mapWidth         = 600
	mapHeight        = 600
	statisticsWidth  = 600
	statisticsHeight = 200
	secondPerYear    = 2.0
	minYear          = 1950
	maxYear          = 2027
	nodata           = -1
)

type ScreenMap struct {
	geojson                          FranceGeoJSON
	minLong, maxLong, minLat, maxLat float64
	startTime                        time.Time
	logger                           *slog.Logger
	stations                         []StationInfo
	weatherData                      []WeatherData
	selectedStation                  *StationInfo
	selectedStationColor             color.Color
	comparativeStation               *StationInfo
	comparativeStationColor          color.Color
	outlineImage                     *ebiten.Image
	statisticsImage                  *ebiten.Image
	fontFace                         *text.GoTextFace
}

func Run(logger *slog.Logger) {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Météo")

	geojson := readGeoJsonFile(logger)

	minLong, maxLong, minLat, maxLat := minMaxLongLat(geojson)
	screenMap := &ScreenMap{
		geojson:                 geojson,
		minLong:                 minLong,
		maxLong:                 maxLong,
		minLat:                  minLat,
		maxLat:                  maxLat,
		startTime:               time.Now(),
		logger:                  logger,
		outlineImage:            ebiten.NewImage(mapWidth, mapHeight),
		statisticsImage:         ebiten.NewImage(statisticsWidth, statisticsHeight),
		selectedStationColor:    color.RGBA{255, 0, 255, 255},
		comparativeStationColor: color.RGBA{0, 255, 0, 255},
		fontFace: &text.GoTextFace{
			Source: components.FontSource,
			Size:   14,
		},
	}

	weather27 := ReadRRTVentFile(logger, fmt.Sprintf("Q_%s_previous-1950-2024_RR-T-Vent.csv", "27"))
	weather35 := ReadRRTVentFile(logger, fmt.Sprintf("Q_%s_previous-1950-2024_RR-T-Vent.csv", "35"))
	weather35Latest := ReadRRTVentFile(logger, fmt.Sprintf("Q_%s_latest-2025-2026_RR-T-Vent.csv", "35"))
	weather85 := ReadRRTVentFile(logger, fmt.Sprintf("Q_%s_previous-1950-2024_RR-T-Vent.csv", "85"))
	screenMap.weatherData = slices.Concat(weather27, weather35, weather35Latest, weather85)
	screenMap.stations = getStationList(screenMap.weatherData)

	screenMap.drawFranceOutline()
	screenMap.statisticsImage.Fill(color.RGBA{255, 255, 255, 255})

	if err := ebiten.RunGame(screenMap); err != nil {
		log.Fatal(err)
	}

}

func (sm *ScreenMap) Update() error {
	mx, my := ebiten.CursorPosition()
	if sm.isCursorInMapView(mx, my) {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			sm.selectedStation = sm.getNearestStation(float64(mx), float64(my))

		}
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			sm.comparativeStation = sm.getNearestStation(float64(mx), float64(my))

		}
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
			sm.redrawStatistics()
		}
	} else if sm.isCursorInStatisticView(mx, my) {
		sm.redrawStatistics()
		sm.drawStatisticForHoveredYear(mx)
	}
	return nil
}

func (sm *ScreenMap) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 255, 255})
	textOp := &text.DrawOptions{}
	textOp.GeoM.Translate(10, 10)
	textOp.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, "France", sm.fontFace, textOp)

	// ebitenutil.DebugPrintAt(screen, "France", 10, 10)
	if sm.selectedStation != nil {
		sm.displayStationInfo(screen, sm.selectedStation, 0, sm.selectedStationColor)
	}
	if sm.comparativeStation != nil {
		sm.displayStationInfo(screen, sm.comparativeStation, 30, sm.comparativeStationColor)
	}
	screen.DrawImage(sm.outlineImage, nil)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, float64(mapHeight))
	screen.DrawImage(sm.statisticsImage, op)
}

func (sm *ScreenMap) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (sm *ScreenMap) drawFranceOutline() {
	var prevX, prevY float64

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

				vector.StrokeLine(sm.outlineImage, float32(prevX), float32(prevY), float32(x), float32(y), 2.0, lineColor, true)

				prevX = x
				prevY = y
			}
		}
	}

}

func (sm *ScreenMap) redrawStatistics() {
	sm.statisticsImage.Clear()
	sm.statisticsImage.Fill(color.RGBA{255, 255, 255, 255})
	sm.displayStationRainGraph(sm.selectedStation, sm.comparativeStation, sm.selectedStationColor)
	sm.displayStationRainGraph(sm.comparativeStation, sm.selectedStation, sm.comparativeStationColor)
	sm.drawXAxisTicks()
	sm.drawYAxisTicks()
}

func (sm *ScreenMap) drawStatisticForHoveredYear(mx int) {
	year := int(math.Round(minYear + float64(mx)*(float64(maxYear-minYear)/float64(statisticsWidth))))
	var rainSelectedStation, rainComparativeStation float64
	if sm.selectedStation != nil {
		rainByYear := rainByYearForStation(sm.selectedStation, sm.weatherData)
		rainSelectedStation = rainByYear[year]
	}
	if sm.comparativeStation != nil {
		rainByYear := rainByYearForStation(sm.comparativeStation, sm.weatherData)
		rainComparativeStation = rainByYear[year]
	}

	yearTextOp := &text.DrawOptions{}
	yearTextOp.GeoM.Translate(float64(mx), 20)
	yearTextOp.ColorScale.ScaleWithColor(color.RGBA{0, 0, 255, 255})
	text.Draw(sm.statisticsImage, fmt.Sprintf("%d", year), sm.fontFace, yearTextOp)

	vector.StrokeLine(sm.statisticsImage, float32(mx), 0, float32(mx), statisticsHeight, 1, color.RGBA{0, 0, 255, 255}, false)

	if rainSelectedStation > rainComparativeStation {
		if rainSelectedStation != 0 {
			textOp := &text.DrawOptions{}
			textOp.GeoM.Translate(float64(mx), 50.0)
			textOp.ColorScale.ScaleWithColor(sm.selectedStationColor)
			text.Draw(sm.statisticsImage, fmt.Sprintf("%.0f", rainSelectedStation), sm.fontFace, textOp)
		}
		if rainComparativeStation != 0 {
			textOp := &text.DrawOptions{}
			textOp.GeoM.Translate(float64(mx), 150.0)
			textOp.ColorScale.ScaleWithColor(sm.comparativeStationColor)
			text.Draw(sm.statisticsImage, fmt.Sprintf("%.0f", rainComparativeStation), sm.fontFace, textOp)
		}
	} else {
		if rainSelectedStation != 0 {
			textOp := &text.DrawOptions{}
			textOp.GeoM.Translate(float64(mx), 150.0)
			textOp.ColorScale.ScaleWithColor(sm.selectedStationColor)
			text.Draw(sm.statisticsImage, fmt.Sprintf("%.0f", rainSelectedStation), sm.fontFace, textOp)
		}
		if rainComparativeStation != 0 {
			textOp := &text.DrawOptions{}
			textOp.GeoM.Translate(float64(mx), 50.0)
			textOp.ColorScale.ScaleWithColor(sm.comparativeStationColor)
			text.Draw(sm.statisticsImage, fmt.Sprintf("%.0f", rainComparativeStation), sm.fontFace, textOp)
		}
	}
}

func (sm *ScreenMap) isCursorInStatisticView(x, y int) bool {
	return y > mapHeight && y < screenHeight
}

func (sm *ScreenMap) isCursorInMapView(x, y int) bool {
	return y < mapHeight
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
	x := (long - sm.minLong) * mapWidth / (sm.maxLong - sm.minLong)
	y := mapHeight - (lat-sm.minLat)*mapHeight/(sm.maxLat-sm.minLat)

	return x, y
}

func (sm *ScreenMap) getLongLatFromScreenPosition(x, y float64) (float64, float64) {
	long := sm.minLong + x*(sm.maxLong-sm.minLong)/mapWidth
	lat := sm.minLat + (mapHeight-y)*(sm.maxLat-sm.minLat)/mapHeight

	return long, lat
}

func (sm *ScreenMap) displayStationInfo(screen *ebiten.Image, station *StationInfo, offset float64, graphColor color.Color) {
	if sm.selectedStation != nil {
		var textX, textY float64
		for _, post := range sm.weatherData {
			if post.NumPost == station.NumPost {
				textX, textY = sm.getScreenPosition(post.Lon, post.Lat)
			}
		}

		vector.FillCircle(screen, float32(math.Floor(textX)), float32(math.Floor(textY)), 2, graphColor, true)

		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(math.Floor(textX), math.Floor(textY+10+offset))
		textOp.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, fmt.Sprintf("Station %s", station.CommonName), sm.fontFace, textOp)
		textOp.GeoM.Reset()
	}

}

func getFirstLastObsDateForStation(station string, weatherData []WeatherData) (time.Time, time.Time) {
	min := time.Date(maxYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	max := time.Date(minYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	for i := range weatherData {
		if weatherData[i].Rain != nodata {
			if weatherData[i].NumPost == station {
				if weatherData[i].ObsDate.Before(min) {
					min = weatherData[i].ObsDate
				} else if weatherData[i].ObsDate.After(max) {
					max = weatherData[i].ObsDate
				}
			}
		}
	}

	return min, max
}

func getStationList(wd []WeatherData) []StationInfo {
	stationMap := make(map[string]StationInfo)
	for i := range wd {
		if _, exist := stationMap[wd[i].NumPost]; !exist {
			stationMap[wd[i].NumPost] = wd[i].StationInfo
		}
	}

	return slices.Collect(maps.Values(stationMap))
}

func (sm *ScreenMap) getNearestStation(x, y float64) *StationInfo {
	long, lat := sm.getLongLatFromScreenPosition(x, y)

	minD := math.MaxFloat64
	var closestStation StationInfo
	for _, station := range sm.stations {
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

func (sm *ScreenMap) drawYAxisTicks() {
	minRain := math.MaxFloat64
	maxRain := -math.MaxFloat64
	if sm.selectedStation != nil {
		min, max := minMaxRainByStation(sm.selectedStation, sm.weatherData)
		if min < minRain {
			minRain = min
		}
		if max > maxRain {
			maxRain = max
		}
	}
	if sm.comparativeStation != nil {
		min, max := minMaxRainByStation(sm.comparativeStation, sm.weatherData)
		if min < minRain {
			minRain = min
		}
		if max > maxRain {
			maxRain = max
		}
	}

	// our scale is from closest 50's below minValue and closest 50's above maxValue
	step := 50.0
	deltaRain := (math.Ceil(maxRain/step) * step) - (math.Floor(minRain/step) * step)

	ratioPxRain := statisticsHeight / deltaRain

	for i := range int(deltaRain / step) {
		y := statisticsHeight - float64(i)*step*ratioPxRain
		vector.StrokeLine(sm.statisticsImage, 0, float32(y), 3, float32(y), 2, color.RGBA{0, 0, 255, 255}, true)
		ebitenutil.DebugPrintAt(sm.statisticsImage, fmt.Sprintf("%.0f", math.Round((math.Floor(minRain/step)*step)+float64(i)*step)), 3, int(y))
	}
}

func (sm *ScreenMap) drawXAxisTicks() {
	// tick every 5 years
	step := 5
	textOp := &text.DrawOptions{}
	textOp.ColorScale.ScaleWithColor(color.White)

	for i := range int(math.Ceil(float64(maxYear-minYear) / float64(step))) {
		x := float32(i*step) * (float32(statisticsWidth) / float32(maxYear-minYear))
		vector.StrokeLine(sm.statisticsImage, x, statisticsHeight, x, statisticsHeight-10, 2, color.RGBA{0, 0, 255, 255}, true)
		textOp.GeoM.Translate(float64(x), 10)
		ebitenutil.DebugPrintAt(sm.statisticsImage, fmt.Sprintf("%d", (50+i*step)%100), int(x), statisticsHeight-20)
	}
}

func minMaxRainByStation(station *StationInfo, weatherData []WeatherData) (float64, float64) {
	rainByYear := rainByYearForStation(station, weatherData)
	// minD, maxD := getFirstLastObsDateForStation(station.NumPost, sm.weatherData)
	minRain := math.MaxFloat64
	maxRain := -math.MaxFloat64

	for _, rain := range rainByYear {
		if rain != nodata {
			if rain < minRain {
				minRain = rain
			}
			if rain > maxRain {
				maxRain = rain
			}
		}
	}

	return minRain, maxRain
}

func (sm *ScreenMap) displayStationRainGraph(station *StationInfo, comparativeStation *StationInfo, graphColor color.Color) {
	if station != nil {
		rainByYear := rainByYearForStation(station, sm.weatherData)
		minD := minYear
		maxD := maxYear

		minRain, maxRain := minMaxRainByStation(station, sm.weatherData)
		if comparativeStation != nil {
			cMinRain, cMaxRain := minMaxRainByStation(comparativeStation, sm.weatherData)
			if cMinRain < minRain {
				minRain = cMinRain
			}
			if cMaxRain > maxRain {
				maxRain = cMaxRain
			}
		}

		var prevX, prevY float32
		for dyear := range maxD - minD {
			currentYear := minD + dyear
			if rain, exist := rainByYear[currentYear]; exist {
				x := float32(statisticsWidth) / (float32(maxD) - float32(minD)) * float32(dyear)
				y := statisticsHeight - statisticsHeight/(float32(maxRain-minRain))*float32(rain-minRain)

				if prevY != nodata && prevY != 0 && y != nodata && y != 0 {
					vector.StrokeLine(sm.statisticsImage, prevX, prevY, x, y, 2, graphColor, true)
				}
				prevX = x
				prevY = y
			}
		}

	}
}

func rainByYearForStation(station *StationInfo, winfos []WeatherData) map[int]float64 {
	rainByYear := make(map[int]float64)
	obsByYear := make(map[int]int)
	// at least 95% of obs should have been done to be a valid measure
	minimalObsByYear := 365.0 * 0.95
	for _, data := range winfos {
		if data.StationInfo.NumPost == station.NumPost {
			rainByYear[data.ObsDate.Year()] += data.Rain
			obsByYear[data.ObsDate.Year()]++
		}
	}

	rainByYearCleaned := make(map[int]float64)

	for i, v := range rainByYear {
		if obsByYear[i] >= int(minimalObsByYear) {
			rainByYearCleaned[i] = v
		} else {
			rainByYearCleaned[i] = -1
		}
	}

	return rainByYearCleaned
}

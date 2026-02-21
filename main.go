package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log/slog"
	"math"
	"meteo/components"
	"meteo/types"
	"os"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/fogleman/gg"
)

type Dimension struct {
	width, height float64
}

const (
	mapHeight = 600
	mapWidth  = 600
)

func main() {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	logger := slog.New(slog.NewJSONHandler(file, nil))

	logger.Info("Démarrage de l'App")

	db, err := types.InitDB("meteo.db", false)
	if err != nil {
		logger.Error("Can't init database", "error", err)
		panic(err)
	}

	a := app.New()
	w := a.NewWindow("Météo")

	geojson := readGeoJsonFile(logger)
	geoData := &types.GeoData{
		FranceGeoJSON: geojson,
	}

	stations := make([]types.StationModel, 0, 100)

	w.Resize(fyne.NewSize(500, 500))

	mapDimension := Dimension{width: float64(mapWidth), height: float64(mapHeight)}
	geoData.Bounds = geoData.GetBounds()
	mapImg := renderMap(geoData, mapDimension)
	interactiveMap := components.NewInteractiveMap(mapImg, mapWidth, mapHeight)

	stationNames := binding.NewStringList()
	selectStation := widget.NewSelectEntry([]string{})

	stationNames.AddListener(binding.NewDataListener(func() {
		names, _ := stationNames.Get()
		selectStation.SetOptions(names)
	}))

	refreshMap := func() {
		stationsImg := renderStations(stations, mapDimension, *geoData.Bounds)
		interactiveMap.AddLayer(stationsImg)

		stationNames.Set(stationsNameList(stations))
	}

	loadDepartment := func(department string) {
		progress := dialog.NewCustomWithoutButtons("Import en cours",
			container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Import du département %s...", department)),
				widget.NewProgressBarInfinite(),
			), w)
		progress.Show()

		go func() {
			err := types.ImportWeatherData(db, logger, department)
			if len(department) == 1 {
				department = "0" + department
			}

			if err != nil {
				fyne.Do(func() {
					progress.Hide()
					dialog.ShowError(err, w)
				})
				return
			}
			stations = types.GetStationsForDepartment(db, department)
			fyne.Do(func() {
				refreshMap()
				progress.Hide()
				dialog.ShowInformation("Import terminé", fmt.Sprintf("Département %s importé avec succès", department), w)
			})
		}()
	}

	sidebar := container.NewVBox(
		widget.NewButton("Charger un département", func() {
			entry := widget.NewEntry()
			entry.SetPlaceHolder("Numéro du département (ex: 35)")
			dialog.ShowForm("Charger un département", "Importer", "Annuler", []*widget.FormItem{
				widget.NewFormItem("Département", entry),
			}, func(ok bool) {
				if ok && entry.Text != "" {
					loadDepartment(strings.TrimSpace(entry.Text))
				}
			}, w)
		}),
		selectStation,
	)

	// var metadataContainer *fyne.Container
	// showStation := func(name string) {
	// 	station, err := getStationByName(stations, name)
	// 	if err == nil {
	// 		sidebar.Remove(metadataContainer)
	// 		metadataContainer = renderStationMetadata(station, weatherData)
	// 		sidebar.Add(metadataContainer)
	// 	}
	// }

	// selectStation.OnChanged = func(v string) {
	// 	selectStation.SetOptions(findStationsByPrefix(stationNames, v))
	// 	showStation(v)
	// }

	// selectStation.OnSubmitted = func(v string) {
	// 	showStation(v)
	// }

	// interactiveMap.OnHover = func(pos fyne.Position) string {
	// 	if len(stations) == 0 {
	// 		return ""
	// 	}
	// 	long, lat := projectionFromXY(
	// 		mapDimension,
	// 		*geoData.Bounds,
	// 		float64(pos.X),
	// 		float64(pos.Y))
	// 	station := weatherData.ClosestStation(long, lat)

	// 	return station.CommonName
	// }
	// interactiveMap.OnTap = func(pos fyne.Position) {
	// 	if len(weatherData.Stations) == 0 {
	// 		return
	// 	}
	// 	long, lat := projectionFromXY(
	// 		mapDimension,
	// 		*geoData.Bounds,
	// 		float64(pos.X),
	// 		float64(pos.Y))

	// 	station := weatherData.ClosestStation(long, lat)

	// 	sidebar.Remove(metadataContainer)
	// 	metadataContainer = renderStationMetadata(station, weatherData)
	// 	sidebar.Add(metadataContainer)
	// }

	split := container.NewHSplit(sidebar, interactiveMap)
	split.Offset = 0.33

	w.SetContent(split)

	w.ShowAndRun()

}

func readGeoJsonFile(logger *slog.Logger) types.FranceGeoJSON {
	filepath := "./data/metropole-version-simplifiee.geojson"
	file, err := os.ReadFile(filepath)
	if err != nil {
		logger.Error("Can't read file", "error", err, "filepath", filepath)
	}

	geojson := types.FranceGeoJSON{}
	err = json.Unmarshal(file, &geojson)
	if err != nil {
		logger.Error("Can't parse to json", "error", err)
	}
	return geojson
}

func projectionFromXY(mapSize Dimension, dataBounds types.Bounds, x, y float64) (long, lat float64) {
	long = dataBounds.MinLong + ((dataBounds.MaxLong - dataBounds.MinLong) / mapSize.width * x)
	lat = dataBounds.MinLat + ((dataBounds.MaxLat - dataBounds.MinLat) / mapSize.height * (mapSize.height - y))

	return long, lat
}

func renderMap(g *types.GeoData, d Dimension) *canvas.Image {
	var prevX, prevY int

	dc := gg.NewContext(int(d.width), int(d.height))
	dc.SetColor(color.White)
	dc.SetLineWidth(2)

	mapDimension := Dimension{width: float64(mapWidth), height: float64(mapHeight)}

	for i := range g.Geometry.Coordinates {
		for j := range g.Geometry.Coordinates[i] {
			for k := range g.Geometry.Coordinates[i][j] {
				outline := g.Geometry.Coordinates[i][j][k]

				x, y := projection(outline[0], outline[1], mapDimension, *g.Bounds)
				if k != 0 {
					dc.MoveTo(float64(prevX), float64(prevY))
					dc.LineTo(float64(x), float64(y))
				}
				prevX, prevY = x, y

			}
			dc.ClosePath()
			dc.Stroke()
		}
	}

	img := canvas.NewImageFromImage(dc.Image())
	img.FillMode = canvas.ImageFillContain
	return img
}

func renderStations(stations []types.StationModel, d Dimension, b types.Bounds) *canvas.Image {

	dc := gg.NewContext(int(d.width), int(d.height))
	dc.SetColor(color.White)
	dc.SetLineWidth(2)

	for _, station := range stations {
		x, y := projection(station.Long, station.Lat, d, b)
		dc.DrawCircle(float64(x), float64(y), 1)
		dc.Fill()
	}

	img := canvas.NewImageFromImage(dc.Image())
	img.FillMode = canvas.ImageFillContain
	return img
}

func projection(long, lat float64, d Dimension, b types.Bounds) (x, y int) {
	x = int(math.Round((long - b.MinLong) * d.width / (b.MaxLong - b.MinLong)))
	y = int(math.Round(d.height - (lat-b.MinLat)*d.height/(b.MaxLat-b.MinLat)))

	return x, y
}

func stationsNameList(stations []types.StationModel) []string {
	names := make([]string, 0, len(stations))

	for _, station := range stations {
		names = append(names, station.CommonName)
	}

	return names
}

func findStationsByPrefix(stations []string, prefix string) []string {
	matches := make([]string, 0, 10)
	for _, station := range stations {
		if strings.HasPrefix(station, prefix) {
			matches = append(matches, station)
		}
	}

	return matches
}

func renderStationMetadata(station *types.StationInfo, weatherData *types.WeatherData) *fyne.Container {
	grid := container.New(layout.NewGridLayout(2))

	nameLabel := widget.NewLabel("Nom")
	name := widget.NewLabel(truncate(station.CommonName, 10))

	min, max, avg := getMinMaxAvgRainByStation(station, *weatherData)

	grid.Add(nameLabel)
	grid.Add(name)

	grid.Add(widget.NewLabel("Moyenne"))
	grid.Add(widget.NewLabel(fmt.Sprintf("%.0f", avg)))

	grid.Add(widget.NewLabel("Min"))
	grid.Add(widget.NewLabel(fmt.Sprintf("%.0f", min)))

	grid.Add(widget.NewLabel("Max"))
	grid.Add(widget.NewLabel(fmt.Sprintf("%.0f", max)))

	return grid
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	truncated := slices.Clone(runes[:max])

	return string(truncated) + "..."
}

func getStationByName(stations []types.StationModel, name string) (*types.StationModel, error) {
	for _, station := range stations {
		if strings.EqualFold(station.CommonName, name) {
			return &station, nil
		}
	}

	return nil, fmt.Errorf("Can't find station %s", name)
}

func getMinMaxAvgRainByStation(station *types.StationInfo, weatherData types.WeatherData) (minRain, maxRain, avgRain float64) {
	type DataPerYear struct {
		occurence int
		rain      float64
	}
	sumPerYear := make(map[int]DataPerYear)
	for _, record := range weatherData.Data {
		if record.NumPost == station.NumPost {
			sumPerYear[record.ObsDate.Year()] = DataPerYear{
				occurence: sumPerYear[record.ObsDate.Year()].occurence + 1,
				rain:      sumPerYear[record.ObsDate.Year()].rain + record.Rain,
			}
		}
	}

	minRain = math.MaxFloat64
	maxRain = -math.MaxFloat64
	completeYear := 0
	sumRain := 0.0
	for _, data := range sumPerYear {
		if data.occurence >= int(math.Round(365*0.95)) {
			if data.rain < minRain {
				minRain = data.rain
			}
			if data.rain > maxRain {
				maxRain = data.rain
			}
			completeYear++
			sumRain += data.rain
		}
	}

	return minRain, maxRain, sumRain / float64(completeYear)

}

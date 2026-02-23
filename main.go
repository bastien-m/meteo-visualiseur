package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"image/color"
	"log/slog"
	"math"
	"meteo/common"
	"meteo/components/ui"
	"meteo/data"
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

const (
	mapHeight = 600
	mapWidth  = 600
)

func createLogger() *slog.Logger {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	logger := slog.New(slog.NewJSONHandler(file, nil))

	return logger
}

func buildInteractiveMap(geoData *data.GeoData) *ui.InteractiveMap {
	mapDimension := common.Dimension{Width: float64(mapWidth), Height: float64(mapHeight)}
	geoData.Bounds = geoData.GetBounds()
	mapImg := renderMap(geoData, mapDimension)
	return ui.NewInteractiveMap(mapImg, mapWidth, mapHeight)
}

func refreshUIWithNewData(
	interactiveMap *ui.InteractiveMap,
	stationBindings binding.List[string],
	stations []types.StationInfo,
	bounds types.Bounds,
) {
	mapDimension := Dimension{width: float64(mapWidth), height: float64(mapHeight)}
	stationsImg := renderStations(stations, mapDimension, bounds)
	interactiveMap.AddLayer(stationsImg)

	stationBindings.Set(stationsNameList(stations))
}

func HandleLoadDepartment(db *sql.DB, stations *[]types.StationInfo, department string, w fyne.Window) {
	progress := dialog.NewCustomWithoutButtons("Import en cours",
		container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Import du département %s...", department)),
			widget.NewProgressBarInfinite(),
		), w)
	progress.Show()

	go func() {
		if len(department) == 1 {
			department = "0" + department
		}
		err := types.DownloadParquetFile(department)

		if err != nil {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(err, w)
			})
			return
		}
		newStations, err := types.GetStations(db)
		if err != nil {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(err, w)
			})
			return
		}

		stations = slices.Concat(stations, newStations)

		fyne.Do(func() {
			refreshUIWithNewData(interactiveMap, stationNames, stations, *geoData.Bounds)
			progress.Hide()
			dialog.ShowInformation("Import terminé", fmt.Sprintf("Département %s importé avec succès", department), w)
		})
	}()
}

func main() {
	logger := createLogger()

	logger.Info("Démarrage de l'App")

	db, err := types.InitDuckDB()
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
	interactiveMap := buildInteractiveMap(geoData)

	w.Resize(fyne.NewSize(500, 500))

	stations := make([]types.StationInfo, 0, 100)
	stationNames := binding.NewStringList()
	selectStation := widget.NewSelectEntry([]string{})
	stationNames.AddListener(binding.NewDataListener(func() {
		names, _ := stationNames.Get()
		selectStation.SetOptions(names)
	}))

	loadDepartment := func(department string) {
		progress := dialog.NewCustomWithoutButtons("Import en cours",
			container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Import du département %s...", department)),
				widget.NewProgressBarInfinite(),
			), w)
		progress.Show()

		go func() {
			if len(department) == 1 {
				department = "0" + department
			}
			err := types.DownloadParquetFile(department)

			if err != nil {
				fyne.Do(func() {
					progress.Hide()
					dialog.ShowError(err, w)
				})
				return
			}
			newStations, err := types.GetStations(db)
			if err != nil {
				fyne.Do(func() {
					progress.Hide()
					dialog.ShowError(err, w)
				})
				return
			}

			stations = slices.Concat(stations, newStations)

			fyne.Do(func() {
				refreshUIWithNewData(interactiveMap, stationNames, stations, *geoData.Bounds)
				progress.Hide()
				dialog.ShowInformation("Import terminé", fmt.Sprintf("Département %s importé avec succès", department), w)
			})
		}()
	}

	loadExistingData := func() {
		progress := dialog.NewCustomWithoutButtons("Import en cours",
			container.NewVBox(
				widget.NewLabel("Import des données ..."),
				widget.NewProgressBarInfinite(),
			), w)
		progress.Show()

		existingStations, err := types.GetStations(db)
		if err != nil {
			progress.Hide()
			dialog.NewError(err, w)
		} else {
			stations = existingStations
			refreshUIWithNewData(interactiveMap, stationNames, stations, *geoData.Bounds)
			progress.Hide()

			dialog.ShowInformation("Import terminé", "Département(s) importé(s) avec succès", w)
		}
	}

	loadExistingData()

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
		widget.NewLabel("Sélectionnez une station"),
		selectStation,
	)

	mw := container.NewMultipleWindows()

	selectStation.OnChanged = func(v string) {
		names, _ := stationNames.Get()
		selectStation.SetOptions(findStationsByPrefix(names, v))
	}

	selectStation.OnSubmitted = func(v string) {
		station, err := getStationByName(stations, v)
		if err != nil {
			dialog.NewError(err, w)
		} else {
			content := buildStationMetadataDisplay(db, w, station)
			if content != nil {
				wrapped := container.New(layout.NewGridWrapLayout(fyne.NewSize(250, 150)), content)
				iw := container.NewInnerWindow(station.CommonName, wrapped)
				iw.CloseIntercept = func() {
					for i, win := range mw.Windows {
						if win == iw {
							mw.Windows = append(mw.Windows[:i], mw.Windows[i+1:]...)
							break
						}
					}
					mw.Refresh()
				}
				mw.Windows = append(mw.Windows, iw)
				mw.Refresh()
			}
		}
	}

	interactiveMap.OnHover = func(pos fyne.Position) string {
		if len(stations) == 0 {
			return ""
		}
		mapDimension := Dimension{width: float64(mapWidth), height: float64(mapHeight)}
		long, lat := projectionFromXY(
			mapDimension,
			*geoData.Bounds,
			float64(pos.X),
			float64(pos.Y))
		station, err := types.GetClosestStationDuck(db, lat, long)

		if err != nil {
			return ""
		}

		return station.CommonName
	}

	interactiveMap.OnTap = func(pos fyne.Position) {
		mapDimension := Dimension{width: float64(mapWidth), height: float64(mapHeight)}
		long, lat := projectionFromXY(
			mapDimension,
			*geoData.Bounds,
			float64(pos.X),
			float64(pos.Y))
		station, err := types.GetClosestStationDuck(db, lat, long)

		if err != nil {
			dialog.NewError(err, w)
		} else {
			content := buildStationMetadataDisplay(db, w, station)
			if content != nil {
				wrapped := container.New(layout.NewGridWrapLayout(fyne.NewSize(250, 150)), content)
				iw := container.NewInnerWindow(station.CommonName, wrapped)
				iw.CloseIntercept = func() {
					for i, win := range mw.Windows {
						if win == iw {
							mw.Windows = append(mw.Windows[:i], mw.Windows[i+1:]...)
							break
						}
					}
					mw.Refresh()
				}
				mw.Windows = append(mw.Windows, iw)
				mw.Refresh()
			}
		}
	}

	mapWithWindows := container.NewStack(interactiveMap, mw)
	split := container.NewHSplit(sidebar, mapWithWindows)
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

func renderStations(stations []types.StationInfo, d Dimension, b types.Bounds) *canvas.Image {

	dc := gg.NewContext(int(d.width), int(d.height))
	dc.SetColor(color.White)
	dc.SetLineWidth(2)

	for _, station := range stations {
		x, y := projection(station.Lon, station.Lat, d, b)
		dc.DrawCircle(float64(x), float64(y), 0.5)
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

func stationsNameList(stations []types.StationInfo) []string {
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

func buildStationMetadataDisplay(db *sql.DB, w fyne.Window, station *types.StationInfo) *fyne.Container {
	weatherData, err := types.GetRainByStationDuck(db, station.NumPost)

	if err != nil {
		dialog.NewError(err, w)
		return nil
	}

	grid := container.New(layout.NewGridLayout(2))

	nameLabel := widget.NewLabel("Nom")
	name := widget.NewLabel(truncate(station.CommonName, 10))

	min, max, avg := getMinMaxAvgRainByStation(weatherData)

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

func getStationByName(stations []types.StationInfo, name string) (*types.StationInfo, error) {
	for _, station := range stations {
		if strings.EqualFold(station.CommonName, name) {
			return &station, nil
		}
	}

	return nil, fmt.Errorf("Can't find station %s", name)
}

func getMinMaxAvgRainByStation(sumPerYear []types.RainByStation) (minRain, maxRain, avgRain float64) {
	minRain = math.MaxFloat64
	maxRain = -math.MaxFloat64
	sumRain := 0.0
	for _, data := range sumPerYear {
		if data.Rain < minRain {
			minRain = data.Rain
		}
		if data.Rain > maxRain {
			maxRain = data.Rain
		}
		sumRain += data.Rain
	}

	return minRain, maxRain, sumRain / float64(len(sumPerYear))

}

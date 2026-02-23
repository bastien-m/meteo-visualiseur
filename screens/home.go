package screens

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"meteo/common"
	"meteo/data"
	"meteo/screens/home"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type HomeScreen struct {
	logger       *slog.Logger
	db           *sql.DB
	window       fyne.Window
	sidebar      *home.HomeSidebar
	homeMap      *home.HomeMap
	stations     []data.StationInfo
	stationList  binding.List[string]
	mapDimension common.Dimension
}

func InitHomeScreen(logger *slog.Logger, db *sql.DB, w fyne.Window) *HomeScreen {
	stationList := binding.NewStringList()

	sidebar := home.InitHomeSidebar(w, stationList)

	dimension := common.Dimension{
		Width:  600,
		Height: 600,
	}
	homeMap := home.InitHomeMap(logger, dimension)

	h := &HomeScreen{
		logger:       logger,
		db:           db,
		window:       w,
		sidebar:      sidebar,
		homeMap:      homeMap,
		stations:     make([]data.StationInfo, 0, 1000),
		stationList:  stationList,
		mapDimension: dimension,
	}

	sidebar.HandleLoadDepartment = h.loadDepartmentHandler()

	return h
}

func (h *HomeScreen) Render() fyne.CanvasObject {
	iMap := h.homeMap.Render()

	mw := container.NewMultipleWindows()

	showStationWindow := func(station *data.StationInfo) {
		content := buildStationMetadataDisplay(h.db, h.window, station)
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

	iMap.OnHover = func(pos fyne.Position) string {
		if len(h.stations) == 0 {
			return ""
		}
		lon, lat := h.homeMap.ProjectFromXY(float64(pos.X), float64(pos.Y))
		station, err := data.GetClosestStationDuck(h.db, lat, lon)
		if err != nil {
			return ""
		}
		return station.CommonName
	}

	iMap.OnTap = func(pos fyne.Position) {
		lon, lat := h.homeMap.ProjectFromXY(float64(pos.X), float64(pos.Y))
		station, err := data.GetClosestStationDuck(h.db, lat, lon)
		if err != nil {
			dialog.NewError(err, h.window)
			return
		}
		showStationWindow(station)
	}

	h.sidebar.HandleSelectStation = func(name string) {
		station, err := getStationByName(h.stations, name)
		if err != nil {
			dialog.NewError(err, h.window)
			return
		}
		showStationWindow(station)
	}

	mapWithWindows := container.NewStack(iMap, mw)
	split := container.NewHSplit(h.sidebar.Render(), mapWithWindows)
	split.Offset = 0.33

	return split
}

func (h *HomeScreen) LoadExistingData() {
	existingStations, err := data.GetStations(h.db)
	if err != nil {
		h.logger.Error("Failed to load existing stations", "error", err)
		return
	}
	h.stations = existingStations
	h.refreshUI()
}

func (h *HomeScreen) refreshUI() {
	h.homeMap.AddStationsLayer(h.stations)
	h.stationList.Set(stationsNameList(h.stations))
}

func (h *HomeScreen) loadDepartmentHandler() func(string) {
	return func(dpt string) {
		if len(dpt) == 1 {
			dpt = "0" + dpt
		}
		progress := dialog.NewCustomWithoutButtons(
			"Import en cours",
			container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Import du département %s...", dpt)),
				widget.NewProgressBarInfinite(),
			),
			h.window,
		)
		progress.Show()

		abort := func(err error) {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(err, h.window)
			})
		}

		go func() {
			err := data.DownloadParquetFile(dpt)
			if err != nil {
				abort(err)
				return
			}
			newStations, err := data.GetStations(h.db)
			if err != nil {
				abort(err)
				return
			}

			h.stations = slices.Concat(h.stations, newStations)

			fyne.Do(func() {
				h.refreshUI()
				progress.Hide()
				dialog.ShowInformation("Import terminé", fmt.Sprintf("Département %s importé avec succès", dpt), h.window)
			})
		}()
	}
}

func stationsNameList(stations []data.StationInfo) []string {
	names := make([]string, 0, len(stations))
	for _, station := range stations {
		names = append(names, station.CommonName)
	}
	return names
}

func getStationByName(stations []data.StationInfo, name string) (*data.StationInfo, error) {
	for i, station := range stations {
		if strings.EqualFold(station.CommonName, name) {
			return &stations[i], nil
		}
	}
	return nil, fmt.Errorf("Can't find station %s", name)
}

func buildStationMetadataDisplay(db *sql.DB, w fyne.Window, station *data.StationInfo) *fyne.Container {
	weatherData, err := data.GetRainByStationDuck(db, station.NumPost)
	if err != nil {
		dialog.NewError(err, w)
		return nil
	}

	grid := container.New(layout.NewGridLayout(2))
	grid.Add(widget.NewLabel("Nom"))
	grid.Add(widget.NewLabel(truncate(station.CommonName, 10)))

	min, max, avg := getMinMaxAvgRainByStation(weatherData)
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
	return string(runes[:max]) + "..."
}

func getMinMaxAvgRainByStation(sumPerYear []data.RainByStation) (minRain, maxRain, avgRain float64) {
	minRain = math.MaxFloat64
	maxRain = -math.MaxFloat64
	sumRain := 0.0
	for _, d := range sumPerYear {
		if d.Rain < minRain {
			minRain = d.Rain
		}
		if d.Rain > maxRain {
			maxRain = d.Rain
		}
		sumRain += d.Rain
	}
	return minRain, maxRain, sumRain / float64(len(sumPerYear))
}

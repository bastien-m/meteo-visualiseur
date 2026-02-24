package screens

import (
	"database/sql"
	"fmt"
	"log/slog"
	"meteo/common"
	appcontext "meteo/context"
	"meteo/data"
	"meteo/screens/home"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
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

func InitHomeScreen() *HomeScreen {
	stationList := binding.NewStringList()
	appContext := appcontext.GetAppContext()

	sidebar := home.InitHomeSidebar(appContext.W, stationList)

	dimension := common.Dimension{
		Width:  600,
		Height: 600,
	}
	homeMap := home.InitHomeMap(dimension)

	h := &HomeScreen{
		logger:       appContext.Logger,
		db:           appContext.DB,
		window:       appContext.W,
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

	h.sidebar.HandleSelectStation = h.handleSelectStation

	split := container.NewHSplit(
		h.sidebar.Render(),
		iMap,
	)
	split.Offset = 0.33

	return split
}

func (h *HomeScreen) handleSelectStation(name string) {
	station, err := getStationByName(h.stations, name)
	if err != nil {
		dialog.NewError(err, h.window)
		return
	}
	h.homeMap.HandleStationWindow(station)
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

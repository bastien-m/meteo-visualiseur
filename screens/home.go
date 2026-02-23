package screens

import (
	"database/sql"
	"fmt"
	"log/slog"
	"meteo/common"
	"meteo/data"
	"meteo/screens/home"
	"slices"

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

	stations := make([]data.StationInfo, 0, 1000)

	sidebar.HandleLoadDepartment = handleLoadDepartment(db, w, stations)

	return &HomeScreen{
		db:           db,
		window:       w,
		sidebar:      sidebar,
		homeMap:      homeMap,
		stations:     stations,
		mapDimension: dimension,
	}
}

func (h *HomeScreen) Render() *container.Split {
	split := container.NewHSplit(
		h.sidebar.Render(),
		h.homeMap.Render(),
	)

	return split
}

func handleLoadDepartment(db *sql.DB, w fyne.Window, stations []data.StationInfo) func(string) {
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
			w,
		)

		progress.Show()

		abort := func(err error) {
			if err != nil {
				fyne.Do(func() {
					progress.Hide()
					dialog.ShowError(err, w)
				})
			}
		}

		go func() {
			err := data.DownloadParquetFile(dpt)
			abort(err)
			newStations, err := data.GetStations(db)
			abort(err)

			stations = slices.Concat(stations, newStations)

			fyne.Do(func() {

			})
		}()
	}
}

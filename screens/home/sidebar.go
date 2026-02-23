package home

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type HomeSidebar struct {
	window               fyne.Window
	stationList          binding.List[string]
	HandleLoadDepartment func(dpt string)
	HandleSelectStation  func(name string)
}

func InitHomeSidebar(window fyne.Window, stationList binding.List[string]) *HomeSidebar {
	return &HomeSidebar{
		window:      window,
		stationList: stationList,
	}
}

func (hs *HomeSidebar) Render() *fyne.Container {
	selectStation := widget.NewSelectEntry([]string{})
	if hs.stationList != nil {
		hs.stationList.AddListener(binding.NewDataListener(func() {
			stations, _ := hs.stationList.Get()
			selectStation.SetOptions(stations)
		}))
	}

	selectStation.OnChanged = func(v string) {
		stations, _ := hs.stationList.Get()
		selectStation.SetOptions(findStationsByPrefix(stations, v))
	}

	selectStation.OnSubmitted = func(v string) {
		if hs.HandleSelectStation != nil {
			hs.HandleSelectStation(v)
		}
	}

	return container.NewVBox(
		widget.NewButton("Charger un département", func() {
			entry := widget.NewEntry()
			entry.SetPlaceHolder("Numéro du département (ex: 35)")
			dialog.ShowForm("Charger un département", "Importer", "Annuler", []*widget.FormItem{
				widget.NewFormItem("Département", entry),
			}, func(ok bool) {
				if ok && entry.Text != "" {
					if hs.HandleLoadDepartment != nil {
						hs.HandleLoadDepartment(strings.TrimSpace(entry.Text))
					}
				}
			}, hs.window)
		}),
		widget.NewLabel("Sélectionnez une station"),
		selectStation,
	)
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

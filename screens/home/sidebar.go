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

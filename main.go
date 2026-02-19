package main

import (
	"encoding/json"
	"log/slog"
	"meteo/components"
	"meteo/types"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

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

	a := app.New()
	w := a.NewWindow("Météo")

	w.Resize(fyne.NewSize(500, 500))

	sidebar := container.NewVBox(
		widget.NewLabel("Options"),
		widget.NewButton("Charger un département", func() {}),
	)

	geojson := readGeoJsonFile(logger)
	geoData := &types.GeoData{
		FranceGeoJSON: geojson,
	}
	geoData.Bounds = geoData.GetBounds()
	mapImg := geoData.RenderMap(mapWidth, mapHeight)
	interactiveMap := components.NewInteractiveMap(mapImg, mapWidth, mapHeight)

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

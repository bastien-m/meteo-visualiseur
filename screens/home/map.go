package home

import (
	"encoding/json"
	"image/color"
	"log/slog"
	"math"
	"meteo/common"
	"meteo/components/ui"
	"meteo/data"
	"os"

	"fyne.io/fyne/v2/canvas"
	"github.com/fogleman/gg"
)

type HomeMap struct {
	logger       *slog.Logger
	dimension    common.Dimension
	geoData      *data.GeoData
	iMap         *ui.InteractiveMap
	stationLayer *canvas.Image
}

func InitHomeMap(logger *slog.Logger, dimension common.Dimension) *HomeMap {
	return &HomeMap{
		logger:    logger,
		dimension: dimension,
	}
}

func (h *HomeMap) Render() *ui.InteractiveMap {
	geojson := readGeoJsonFile(h.logger)
	geoData := &data.GeoData{
		FranceGeoJSON: geojson,
	}
	geoData.SetBounds()
	h.geoData = geoData

	mapImg := h.renderMap(geoData, h.dimension)
	h.iMap = ui.NewInteractiveMap(mapImg, h.dimension.Width, h.dimension.Height)
	return h.iMap
}

func (h *HomeMap) AddStationsLayer(stations []data.StationInfo) {
	stationsImg := renderStations(stations, h.dimension, *h.geoData.Bounds)
	h.iMap.RemoveLayer(h.stationLayer)
	h.iMap.AddLayer(stationsImg)
	h.stationLayer = stationsImg
}

func (h *HomeMap) ProjectFromXY(x, y float64) (lon, lat float64) {
	return projectionFromXY(h.dimension, *h.geoData.Bounds, x, y)
}

func readGeoJsonFile(logger *slog.Logger) data.FranceGeoJSON {
	filepath := "./data/geo/metropole-version-simplifiee.geojson"
	file, err := os.ReadFile(filepath)
	if err != nil {
		logger.Error("Can't read file", "error", err, "filepath", filepath)
	}

	geojson := data.FranceGeoJSON{}
	err = json.Unmarshal(file, &geojson)
	if err != nil {
		logger.Error("Can't parse to json", "error", err)
	}
	return geojson
}

func (h *HomeMap) renderMap(g *data.GeoData, d common.Dimension) *canvas.Image {
	var prevX, prevY int

	dc := gg.NewContext(int(d.Width), int(d.Height))
	dc.SetColor(color.White)
	dc.SetLineWidth(2)

	for i := range g.Geometry.Coordinates {
		for j := range g.Geometry.Coordinates[i] {
			for k := range g.Geometry.Coordinates[i][j] {
				outline := g.Geometry.Coordinates[i][j][k]

				x, y := projection(outline[0], outline[1], h.dimension, *g.Bounds)
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

func renderStations(stations []data.StationInfo, d common.Dimension, b data.Bounds) *canvas.Image {
	dc := gg.NewContext(int(d.Width), int(d.Height))
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

func projection(long, lat float64, d common.Dimension, b data.Bounds) (x, y int) {
	x = int(math.Round((long - b.MinLong) * d.Width / (b.MaxLong - b.MinLong)))
	y = int(math.Round(d.Height - (lat-b.MinLat)*d.Height/(b.MaxLat-b.MinLat)))
	return x, y
}

func projectionFromXY(d common.Dimension, b data.Bounds, x, y float64) (lon, lat float64) {
	lon = b.MinLong + ((b.MaxLong - b.MinLong) / d.Width * x)
	lat = b.MinLat + ((b.MaxLat - b.MinLat) / d.Height * (d.Height - y))
	return lon, lat
}

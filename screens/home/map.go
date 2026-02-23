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
	logger    *slog.Logger
	dimension common.Dimension
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
	mapImg := h.renderMap(geoData, h.dimension)
	return ui.NewInteractiveMap(mapImg, h.dimension.Width, h.dimension.Height)
}

func readGeoJsonFile(logger *slog.Logger) data.FranceGeoJSON {
	filepath := "./data/metropole-version-simplifiee.geojson"
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

func projection(long, lat float64, d common.Dimension, b data.Bounds) (x, y int) {
	x = int(math.Round((long - b.MinLong) * d.Width / (b.MaxLong - b.MinLong)))
	y = int(math.Round(d.Height - (lat-b.MinLat)*d.Height/(b.MaxLat-b.MinLat)))

	return x, y
}

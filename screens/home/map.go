package home

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"image/color"
	"log/slog"
	"math"
	"meteo/common"
	"meteo/components/ui"
	appcontext "meteo/context"
	"meteo/data"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/fogleman/gg"
)

type HomeMap struct {
	w            fyne.Window
	logger       *slog.Logger
	dimension    common.Dimension
	geoData      *data.GeoData
	iMap         *ui.InteractiveMap
	stationLayer *canvas.Image
	mw           *container.MultipleWindows
	db           *sql.DB
}

func InitHomeMap(dimension common.Dimension) *HomeMap {
	appContext := appcontext.GetAppContext()
	return &HomeMap{
		w:         appContext.W,
		logger:    appContext.Logger,
		db:        appContext.DB,
		dimension: dimension,
		mw:        container.NewMultipleWindows(),
	}
}

func (h *HomeMap) Render() *fyne.Container {
	geojson := readGeoJsonFile(h.logger)
	geoData := &data.GeoData{
		FranceGeoJSON: geojson,
	}
	geoData.SetBounds()
	h.geoData = geoData

	mapImg := h.renderMap(geoData, h.dimension)
	h.iMap = ui.NewInteractiveMap(mapImg, h.dimension.Width, h.dimension.Height)

	h.iMap.OnHover = h.handleMapHovered
	h.iMap.OnTap = h.handleMapTapped

	return container.NewStack(h.iMap, h.mw)
}

func (h *HomeMap) AddStationsLayer(stations []data.StationInfo) {
	stationsImg := renderStations(stations, h.dimension, *h.geoData.Bounds)
	h.iMap.RemoveLayer(h.stationLayer)
	h.iMap.AddLayer(stationsImg)
	h.stationLayer = stationsImg
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

				x, y := common.Projection(outline[0], outline[1], h.dimension, *g.Bounds)
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
		x, y := common.Projection(station.Lon, station.Lat, d, b)
		dc.DrawCircle(float64(x), float64(y), 0.5)
		dc.Fill()
	}

	img := canvas.NewImageFromImage(dc.Image())
	img.FillMode = canvas.ImageFillContain
	return img
}

func (h *HomeMap) handleMapHovered(pos fyne.Position) string {
	lon, lat := common.ProjectionFromXY(float64(pos.X), float64(pos.Y), h.dimension, *h.geoData.Bounds)
	station, err := data.GetClosestStation(h.db, lat, lon)
	if err != nil {
		return ""
	}
	return station.CommonName
}

func (h *HomeMap) handleMapTapped(pos fyne.Position) {
	lon, lat := common.ProjectionFromXY(float64(pos.X), float64(pos.Y), h.dimension, *h.geoData.Bounds)
	station, err := data.GetClosestStation(h.db, lat, lon)
	if err != nil {
		dialog.NewError(err, h.w)
		return
	}
	h.HandleStationWindow(station)
}

func (h *HomeMap) HandleStationWindow(station *data.StationInfo) {
	content := buildStationMetadataDisplay(h.db, h.w, station)
	if content != nil {
		wrapped := container.New(layout.NewGridWrapLayout(fyne.NewSize(250, 150)), content)
		iw := container.NewInnerWindow(station.CommonName, wrapped)
		iw.CloseIntercept = func() {
			for i, win := range h.mw.Windows {
				if win == iw {
					h.mw.Windows = append(h.mw.Windows[:i], h.mw.Windows[i+1:]...)
					break
				}
			}
			h.mw.Refresh()
		}
		h.mw.Windows = append(h.mw.Windows, iw)
		h.mw.Refresh()
	}
}

func buildStationMetadataDisplay(db *sql.DB, w fyne.Window, station *data.StationInfo) *fyne.Container {
	weatherData, err := data.GetRainByStation(db, station.NumPost)
	if err != nil {
		dialog.NewError(err, w)
		return nil
	}

	grid := container.New(layout.NewGridLayout(2))
	grid.Add(widget.NewLabel("Nom"))
	grid.Add(widget.NewLabel(common.Truncate(station.CommonName, 10)))

	min, max, avg := getMinMaxAvgRainByStation(weatherData)
	grid.Add(widget.NewLabel("Moyenne"))
	grid.Add(widget.NewLabel(fmt.Sprintf("%.0f", avg)))

	grid.Add(widget.NewLabel("Min"))
	grid.Add(widget.NewLabel(fmt.Sprintf("%.0f", min)))

	grid.Add(widget.NewLabel("Max"))
	grid.Add(widget.NewLabel(fmt.Sprintf("%.0f", max)))

	return grid
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

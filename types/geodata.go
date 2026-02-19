package types

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2/canvas"
	"github.com/fogleman/gg"
)

type Coordinate []float64

type FranceGeoJSON struct {
	Geometry struct {
		Coordinates [][][]Coordinate `json:"coordinates"`
	} `json:"geometry"`
}

type GeoData struct {
	FranceGeoJSON
	Bounds *Bounds
}

type Bounds struct {
	minLong, maxLong, minLat, maxLat float64
}

func (g *GeoData) GetBounds() *Bounds {
	bounds := &Bounds{
		minLong: math.MaxFloat64,
		maxLong: -math.MaxFloat64,
		minLat:  math.MaxFloat64,
		maxLat:  -math.MaxFloat64,
	}

	for i := range g.Geometry.Coordinates {
		for j := range g.Geometry.Coordinates[i] {
			for k := range g.Geometry.Coordinates[i][j] {
				coordinate := g.Geometry.Coordinates[i][j][k]

				currentLong := coordinate[0]
				currentLat := coordinate[1]

				if currentLong > bounds.maxLong {
					bounds.maxLong = currentLong
				} else if currentLong < bounds.minLong {
					bounds.minLong = currentLong
				}

				if currentLat > bounds.maxLat {
					bounds.maxLat = currentLat
				} else if currentLat < bounds.minLat {
					bounds.minLat = currentLat
				}
			}
		}
	}
	return bounds
}

func (g *GeoData) Projection(long, lat float64, width, height int) (x, y int) {
	x = int(math.Round((long - g.Bounds.minLong) * float64(width) / (g.Bounds.maxLong - g.Bounds.minLong)))
	y = int(math.Round(float64(height) - (lat-g.Bounds.minLat)*float64(height)/(g.Bounds.maxLat-g.Bounds.minLat)))

	return x, y
}

func (g *GeoData) RenderMap(width, height int) *canvas.Image {
	var prevX, prevY int

	dc := gg.NewContext(width, height)
	dc.SetColor(color.White)
	dc.SetLineWidth(2)

	for i := range g.Geometry.Coordinates {
		for j := range g.Geometry.Coordinates[i] {
			for k := range g.Geometry.Coordinates[i][j] {
				outline := g.Geometry.Coordinates[i][j][k]

				x, y := g.Projection(outline[0], outline[1], width, height)
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

package types

import (
	"math"
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
	MinLong, MaxLong, MinLat, MaxLat float64
}

func (g *GeoData) GetBounds() *Bounds {
	bounds := &Bounds{
		MinLong: math.MaxFloat64,
		MaxLong: -math.MaxFloat64,
		MinLat:  math.MaxFloat64,
		MaxLat:  -math.MaxFloat64,
	}

	for i := range g.Geometry.Coordinates {
		for j := range g.Geometry.Coordinates[i] {
			for k := range g.Geometry.Coordinates[i][j] {
				coordinate := g.Geometry.Coordinates[i][j][k]

				currentLong := coordinate[0]
				currentLat := coordinate[1]

				if currentLong > bounds.MaxLong {
					bounds.MaxLong = currentLong
				} else if currentLong < bounds.MinLong {
					bounds.MinLong = currentLong
				}

				if currentLat > bounds.MaxLat {
					bounds.MaxLat = currentLat
				} else if currentLat < bounds.MinLat {
					bounds.MinLat = currentLat
				}
			}
		}
	}
	return bounds
}

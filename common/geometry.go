package common

import (
	"math"
	"meteo/data"
)

func Projection(long, lat float64, c Position, d Dimension, b data.Bounds) (x, y int) {
	x = int(math.Round((long-b.MinLong)*d.Width/(b.MaxLong-b.MinLong)) - c.X)
	y = int(math.Round(d.Height-(lat-b.MinLat)*d.Height/(b.MaxLat-b.MinLat)) + c.Y)
	return x, y
}

func ProjectionFromXY(x, y float64, c Position, d Dimension, b data.Bounds) (lon, lat float64) {
	lon = b.MinLong + ((b.MaxLong - b.MinLong) / d.Width * (x - c.X))
	lat = b.MinLat + ((b.MaxLat - b.MinLat) / d.Height * (d.Height - (y + c.Y)))
	return lon, lat
}

package common

import (
	"meteo/data"
)

func Projection(long, lat float64, c Position, d Dimension, b data.Bounds) (x, y float64) {
	x = ((long - b.MinLong) * d.Width / (b.MaxLong - b.MinLong)) - c.X
	y = (d.Height - (lat-b.MinLat)*d.Height/(b.MaxLat-b.MinLat)) + c.Y
	return x * c.Z, y * c.Z
}

func ProjectionFromXY(x, y float64, c Position, d Dimension, b data.Bounds) (lon, lat float64) {
	lon = b.MinLong + ((b.MaxLong - b.MinLong) / d.Width * (x/c.Z + c.X))
	lat = b.MinLat + ((b.MaxLat - b.MinLat) / d.Height * (d.Height - (y/c.Z - c.Y)))
	return lon, lat
}

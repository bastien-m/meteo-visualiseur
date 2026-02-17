package components

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Button struct {
	X, Y, W, H int
	Label      string
}

func (b *Button) Contains(x, y int) bool {
	return x >= b.X && x <= b.X+b.W && y >= b.Y && y <= b.Y+b.H
}

func (b *Button) Draw(screen *ebiten.Image) {
	// Background
	vector.FillRect(screen, float32(b.X), float32(b.Y), float32(b.W), float32(b.H), color.RGBA{100, 100, 200, 255}, true)
	// Border
	vector.StrokeRect(screen, float32(b.X), float32(b.Y), float32(b.W), float32(b.H), 2, color.RGBA{50, 50, 150, 255}, true)

	face := &text.GoTextFace{
		Source: FontSource,
		Size:   16,
	}
	w, h := text.Measure(b.Label, face, 0)

	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(b.X)+float64(b.W)/2-w/2, float64(b.Y)+float64(b.H)/2-h/2)
	op.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, b.Label, face, op)
}

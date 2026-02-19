package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type InteractiveMap struct {
	widget.BaseWidget
	image   *canvas.Image
	size    fyne.Size
	OnTap   func(pos fyne.Position)
}

func NewInteractiveMap(img *canvas.Image, width, height float32) *InteractiveMap {
	m := &InteractiveMap{image: img, size: fyne.NewSize(width, height)}
	m.ExtendBaseWidget(m)
	return m
}

func (m *InteractiveMap) MinSize() fyne.Size {
	return m.size
}

func (m *InteractiveMap) Tapped(ev *fyne.PointEvent) {
	if m.OnTap != nil {
		m.OnTap(ev.Position)
	}
}

func (m *InteractiveMap) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.image)
}

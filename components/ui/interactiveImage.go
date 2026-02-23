package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type InteractiveMap struct {
	widget.BaseWidget
	image      *canvas.Image
	size       fyne.Size
	layers     []*canvas.Image
	tooltip    *canvas.Text
	hoverTimer *time.Timer
	OnTap      func(pos fyne.Position)
	OnHover    func(pos fyne.Position) string
}

var _ desktop.Hoverable = (*InteractiveMap)(nil)

func NewInteractiveMap(img *canvas.Image, width, height float64) *InteractiveMap {
	tooltip := canvas.NewText("", color.White)
	tooltip.TextSize = 12
	tooltip.Hidden = true

	m := &InteractiveMap{image: img, size: fyne.NewSize(float32(width), float32(height)), tooltip: tooltip}
	m.ExtendBaseWidget(m)
	return m
}

func (m *InteractiveMap) AddLayer(img *canvas.Image) {
	m.layers = append(m.layers, img)
	fmt.Printf("number of layers %d", len(m.layers))
	m.Refresh()
}

func (m *InteractiveMap) RemoveLayer(img *canvas.Image) {
	layerIndex := -1
	for i := range m.layers {
		if m.layers[i] == img {
			layerIndex = i
			break
		}
	}

	if layerIndex != -1 {
		// remove the layer
		copy(m.layers[layerIndex:], m.layers[layerIndex+1:])
		m.layers[len(m.layers)-1] = nil // avoid memory leak
		m.layers = m.layers[:len(m.layers)-1]
	}

}

func (m *InteractiveMap) MinSize() fyne.Size {
	return m.size
}

func (m *InteractiveMap) Tapped(ev *fyne.PointEvent) {
	if m.OnTap != nil {
		m.OnTap(ev.Position)
	}
}

func (m *InteractiveMap) MouseIn(ev *desktop.MouseEvent) {
	m.updateTooltip(ev.Position)
}

func (m *InteractiveMap) MouseMoved(ev *desktop.MouseEvent) {
	m.updateTooltip(ev.Position)
}

func (m *InteractiveMap) MouseOut() {
	m.tooltip.Hidden = true
	m.tooltip.Refresh()
}

func (m *InteractiveMap) updateTooltip(pos fyne.Position) {
	if m.OnHover == nil {
		return
	}
	if m.hoverTimer != nil {
		m.hoverTimer.Stop()
	}
	m.hoverTimer = time.AfterFunc(200*time.Millisecond, func() {
		fyne.Do(func() {
			text := m.OnHover(pos)
			if text == "" {
				m.tooltip.Hidden = true
				m.tooltip.Refresh()
				return
			}
			m.tooltip.Text = text
			m.tooltip.Hidden = false
			m.tooltip.Move(fyne.NewPos(pos.X+10, pos.Y-20))
			m.tooltip.Refresh()
		})
	})
}

func (m *InteractiveMap) CreateRenderer() fyne.WidgetRenderer {
	return &mapRenderer{m: m}
}

type mapRenderer struct {
	m *mapRendererOwner
}

type mapRendererOwner = InteractiveMap

func (r *mapRenderer) Layout(size fyne.Size) {
	r.m.image.Resize(size)
	r.m.image.Move(fyne.NewPos(0, 0))
	for _, l := range r.m.layers {
		l.Resize(size)
		l.Move(fyne.NewPos(0, 0))
	}
}

func (r *mapRenderer) MinSize() fyne.Size {
	return r.m.size
}

func (r *mapRenderer) Refresh() {
	r.m.image.Refresh()
	for _, l := range r.m.layers {
		l.Refresh()
	}
	r.m.tooltip.Refresh()
}

func (r *mapRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.m.image}
	for _, l := range r.m.layers {
		objs = append(objs, l)
	}
	objs = append(objs, r.m.tooltip)
	return objs
}

func (r *mapRenderer) Destroy() {}

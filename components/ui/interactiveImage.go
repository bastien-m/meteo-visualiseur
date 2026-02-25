package ui

import (
	"image/color"
	"meteo/common"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type MapMode int

const (
	NORMAL MapMode = iota
	MOVE
)

type InteractiveMap struct {
	widget.BaseWidget
	Image                *canvas.Image
	size                 fyne.Size
	layers               []*canvas.Image
	tooltip              *canvas.Text
	hoverTimer           *time.Timer
	OnTap                func(pos fyne.Position)
	OnHover              func(pos fyne.Position) string
	dragCursor           *canvas.Image
	panCursor            *canvas.Image
	mapMode              binding.Int
	isDragging           bool
	OnDrag               func(dx, dy float64)
	previousDragPosition common.Position
	isFirstDrag          bool
}

var _ desktop.Hoverable = (*InteractiveMap)(nil)
var _ desktop.Cursorable = (*InteractiveMap)(nil)

func NewInteractiveMap(
	img *canvas.Image,
	width, height float64,
	mapMode binding.Int,
) *InteractiveMap {
	tooltip := canvas.NewText("", color.White)
	tooltip.TextSize = 12
	tooltip.Hidden = true

	cursorSize := fyne.NewSize(16, 16)
	dragCursor := canvas.NewImageFromResource(ResourceDragPng)
	dragCursor.Resize(cursorSize)
	dragCursor.Hidden = true

	panCursor := canvas.NewImageFromResource(ResourcePanPng)
	panCursor.Resize(cursorSize)
	panCursor.Hidden = true

	m := &InteractiveMap{
		Image:                img,
		size:                 fyne.NewSize(float32(width), float32(height)),
		tooltip:              tooltip,
		isDragging:           false,
		dragCursor:           dragCursor,
		panCursor:            panCursor,
		mapMode:              mapMode,
		previousDragPosition: common.Position{X: 0, Y: 0, Z: 1},
		isFirstDrag:          true,
	}
	m.ExtendBaseWidget(m)

	return m
}

func (m *InteractiveMap) Dragged(e *fyne.DragEvent) {
	mode, _ := m.mapMode.Get()
	switch MapMode(mode) {
	case NORMAL:
		m.isDragging = false
		m.dragCursor.Hidden = true
		m.dragCursor.Refresh()
		m.panCursor.Hidden = true
		m.panCursor.Refresh()
	case MOVE:
		m.isDragging = true
		offset := fyne.NewPos(-16, -16)
		cursorPos := e.Position.Add(offset)
		m.dragCursor.Move(cursorPos)
		m.dragCursor.Hidden = false
		m.dragCursor.Refresh()
		m.panCursor.Hidden = true
		m.panCursor.Refresh()

		if m.OnDrag != nil {
			if !m.isFirstDrag {
				dx := float64(e.Position.X) - m.previousDragPosition.X
				dy := float64(e.Position.Y) - m.previousDragPosition.Y
				m.OnDrag(dx, dy)
			}

			m.isFirstDrag = false

			m.previousDragPosition.X = float64(e.Position.X)
			m.previousDragPosition.Y = float64(e.Position.Y)
		}

	}

	// TODO: move camera here
}

func (m *InteractiveMap) DragEnd() {
	m.isFirstDrag = true
	m.isDragging = false
	m.Refresh()
}

func (m *InteractiveMap) Cursor() desktop.Cursor {
	mode, _ := m.mapMode.Get()
	if MapMode(mode) == MOVE {
		return desktop.HiddenCursor
	}
	return desktop.DefaultCursor
}

func (m *InteractiveMap) AddLayer(img *canvas.Image) {
	m.layers = append(m.layers, img)
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
	offset := fyne.NewPos(-16, -16)
	cursorPos := ev.Position.Add(offset)
	currentMapMode, _ := m.mapMode.Get()
	if MapMode(currentMapMode) == MOVE {
		if m.isDragging {
			m.dragCursor.Move(cursorPos)
			m.dragCursor.Hidden = false
			m.dragCursor.Refresh()
			m.panCursor.Hidden = true
			m.panCursor.Refresh()
		} else {
			m.panCursor.Move(cursorPos)
			m.panCursor.Hidden = false
			m.panCursor.Refresh()
			m.dragCursor.Hidden = true
			m.dragCursor.Refresh()
		}
	} else {
		m.updateTooltip(ev.Position)
	}
}

func (m *InteractiveMap) MouseOut() {
	m.dragCursor.Hidden = true
	m.dragCursor.Refresh()
	m.panCursor.Hidden = true
	m.panCursor.Refresh()
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

type mapRendererOwner = InteractiveMap
type mapRenderer struct {
	m *mapRendererOwner
}

func (m *InteractiveMap) CreateRenderer() fyne.WidgetRenderer {
	return &mapRenderer{m: m}
}

func (r *mapRenderer) Layout(size fyne.Size) {
	r.m.Image.Resize(size)
	r.m.Image.Move(fyne.NewPos(0, 0))
	for _, l := range r.m.layers {
		l.Resize(size)
		l.Move(fyne.NewPos(0, 0))
	}
}

func (r *mapRenderer) MinSize() fyne.Size {
	return r.m.size
}

func (r *mapRenderer) Refresh() {
	r.m.Image.Refresh()
	for _, l := range r.m.layers {
		l.Refresh()
	}
	r.m.tooltip.Refresh()
	r.m.dragCursor.Refresh()
	r.m.panCursor.Refresh()
}

func (r *mapRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.m.Image}
	for _, l := range r.m.layers {
		objs = append(objs, l)
	}
	objs = append(objs, r.m.tooltip, r.m.dragCursor, r.m.panCursor)
	return objs
}

func (r *mapRenderer) Destroy() {}

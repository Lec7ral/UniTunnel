package ui

import (
	"fmt"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// RealtimeChart displays real-time bandwidth (MB/s) and ping latency (ms).
type RealtimeChart struct {
	Container   *fyne.Container
	linesGroup  *fyne.Container
	speedPoints []float64
	pingPoints  []float64
	statsLabel  *widget.Label
	maxPoints   int
	width       float32
	height      float32
	mu          sync.Mutex
}

// NewRealtimeChart creates a compact, sleek dark chart matching VPN client designs.
func NewRealtimeChart(width, height float32) *RealtimeChart {
	rc := &RealtimeChart{
		maxPoints:   24,
		width:       width,
		height:      height,
		speedPoints: make([]float64, 24),
		pingPoints:  make([]float64, 24),
	}

	rc.statsLabel = widget.NewLabelWithStyle(
		"Flujo: 0.00 MB/s  ·  Latencia: -- ms",
		fyne.TextAlignCenter,
		fyne.TextStyle{Monospace: true, Bold: true},
	)

	// Sleek dark canvas background with rounded corners and border
	bg := canvas.NewRectangle(color.NRGBA{R: 15, G: 23, B: 42, A: 255}) // Slate-900
	bg.SetMinSize(fyne.NewSize(width, height))
	bg.Resize(fyne.NewSize(width, height))
	bg.StrokeColor = color.NRGBA{R: 51, G: 65, B: 85, A: 255} // Slate-700
	bg.StrokeWidth = 1
	bg.CornerRadius = 6

	rc.linesGroup = container.NewWithoutLayout()
	chartStack := container.NewStack(bg, rc.linesGroup)

	// Legend indicator under chart
	legendSpeed := canvas.NewText("● Flujo (MB/s)", color.NRGBA{R: 52, G: 211, B: 153, A: 255})
	legendSpeed.TextSize = 10
	legendPing := canvas.NewText("● Latencia (ms)", color.NRGBA{R: 56, G: 189, B: 248, A: 220})
	legendPing.TextSize = 10
	legendRow := container.NewHBox(
		container.NewCenter(legendSpeed),
		widget.NewLabel(" "),
		container.NewCenter(legendPing),
	)

	rc.Container = container.NewVBox(
		rc.statsLabel,
		chartStack,
		container.NewCenter(legendRow),
	)

	return rc
}

// AddData pushes a new speed and ping point into the rolling window and updates the canvas.
func (rc *RealtimeChart) AddData(speedMBs float64, pingMS int64) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.speedPoints = append(rc.speedPoints[1:], speedMBs)
	rc.pingPoints = append(rc.pingPoints[1:], float64(pingMS))

	pingStr := "-- ms"
	if pingMS > 0 {
		pingStr = fmt.Sprintf("%d ms", pingMS)
	}
	rc.statsLabel.SetText(fmt.Sprintf("Flujo: %.2f MB/s  ·  Latencia: %s", speedMBs, pingStr))

	rc.redraw()
}

func (rc *RealtimeChart) redraw() {
	rc.linesGroup.Objects = nil

	var maxSpeed float64 = 1.0
	for _, s := range rc.speedPoints {
		if s > maxSpeed {
			maxSpeed = s
		}
	}

	var maxPing float64 = 80.0
	for _, p := range rc.pingPoints {
		if p > maxPing {
			maxPing = p
		}
	}

	w := rc.width
	h := rc.height
	stepX := w / float32(rc.maxPoints-1)

	// Mid baseline grid line
	grid := canvas.NewLine(color.NRGBA{R: 30, G: 41, B: 59, A: 255})
	grid.Position1 = fyne.NewPos(0, h/2)
	grid.Position2 = fyne.NewPos(w, h/2)
	grid.StrokeWidth = 1
	rc.linesGroup.Add(grid)

	// 1. Draw Ping Line (Sky Cyan)
	pingColor := color.NRGBA{R: 56, G: 189, B: 248, A: 190}
	for i := 0; i < len(rc.pingPoints)-1; i++ {
		p1 := math.Max(0, rc.pingPoints[i])
		p2 := math.Max(0, rc.pingPoints[i+1])
		x1 := float32(i) * stepX
		y1 := h - float32((p1/maxPing)*float64(h-10)) - 5
		x2 := float32(i+1) * stepX
		y2 := h - float32((p2/maxPing)*float64(h-10)) - 5
		line := canvas.NewLine(pingColor)
		line.Position1 = fyne.NewPos(x1, y1)
		line.Position2 = fyne.NewPos(x2, y2)
		line.StrokeWidth = 1.5
		rc.linesGroup.Add(line)
	}

	// 2. Draw Speed Line (Emerald Green)
	speedColor := color.NRGBA{R: 52, G: 211, B: 153, A: 255}
	for i := 0; i < len(rc.speedPoints)-1; i++ {
		x1 := float32(i) * stepX
		y1 := h - float32((rc.speedPoints[i]/maxSpeed)*float64(h-10)) - 5
		x2 := float32(i+1) * stepX
		y2 := h - float32((rc.speedPoints[i+1]/maxSpeed)*float64(h-10)) - 5
		line := canvas.NewLine(speedColor)
		line.Position1 = fyne.NewPos(x1, y1)
		line.Position2 = fyne.NewPos(x2, y2)
		line.StrokeWidth = 2.2
		rc.linesGroup.Add(line)
	}

	rc.linesGroup.Refresh()
}

package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var IconPNG []byte

//go:embed icon.ico
var IconICO []byte

// AppIconResource provides the embedded PNG resource for Fyne window and tray icons.
var AppIconResource = fyne.NewStaticResource("icon.png", IconPNG)

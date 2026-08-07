package ui

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"charm.land/lipgloss/v2"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// renderHalfBlocks draws an image as rows of ▀ cells, the top pixel as the
// foreground color and the bottom pixel as the background, so each text row
// carries two pixel rows. A terminal cell is roughly twice as tall as it is
// wide, which makes those two stacked pixels close to square.
func renderHalfBlocks(img image.Image, maxCols, maxRows int) []string {
	if maxCols < 1 || maxRows < 1 {
		return nil
	}
	srcW := img.Bounds().Dx()
	srcH := img.Bounds().Dy()
	if srcW < 1 || srcH < 1 {
		return nil
	}
	cols := maxCols
	rows := (srcH*cols + 2*srcW - 1) / (2 * srcW)
	if rows > maxRows {
		rows = maxRows
		cols = 2 * rows * srcW / srcH
		if cols < 1 {
			cols = 1
		}
	}
	if rows < 1 {
		rows = 1
	}

	scaled := image.NewRGBA(image.Rect(0, 0, cols, 2*rows))
	xdraw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), img, img.Bounds(), xdraw.Src, nil)

	lines := make([]string, 0, rows)
	var b strings.Builder
	for y := 0; y < rows; y++ {
		b.Reset()
		for x := 0; x < cols; x++ {
			top := opaque(scaled.RGBAAt(x, 2*y))
			bottom := opaque(scaled.RGBAAt(x, 2*y+1))
			b.WriteString(lipgloss.NewStyle().Foreground(top).Background(bottom).Render("▀"))
		}
		lines = append(lines, b.String())
	}
	return lines
}

// opaque composites a pixel over black so transparent sticker regions render
// as dark cells instead of leaking the alpha into the terminal colors.
// image.RGBA stores alpha-premultiplied channels, so over black this is just
// forcing the alpha to full.
func opaque(c color.RGBA) color.RGBA {
	c.A = 0xff
	return c
}

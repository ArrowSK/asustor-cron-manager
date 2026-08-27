package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	out := flag.String("out", "icon.png", "output PNG")
	size := flag.Int("size", 90, "output size")
	flag.Parse()
	img := render(*size, 4)
	f, err := os.Create(*out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func render(size, scale int) image.Image {
	w := size * scale
	hi := image.NewRGBA(image.Rect(0, 0, w, w))
	bg := color.RGBA{246, 248, 251, 255}
	ink := color.RGBA{32, 49, 68, 255}
	teal := color.RGBA{31, 143, 138, 255}
	blue := color.RGBA{49, 95, 159, 255}
	roundedRect(hi, int(.07*float64(w)), int(.07*float64(w)), int(.93*float64(w)), int(.93*float64(w)), int(.21*float64(w)), bg)
	cx, cy := int(.405*float64(w)), int(.49*float64(w))
	r := int(.225 * float64(w))
	sw := int(.058 * float64(w))
	circleStrokeGradient(hi, cx, cy, r, sw, teal, blue)
	line(hi, cx, int(.36*float64(w)), cx, int(.505*float64(w)), int(.05*float64(w)), ink)
	line(hi, cx, int(.505*float64(w)), int(.51*float64(w)), int(.575*float64(w)), int(.05*float64(w)), ink)
	circleFill(hi, cx, int(.505*float64(w)), int(.027*float64(w)), ink)
	rows := []float64{.32, .455, .59, .725}
	starts := []float64{.64, .68, .68, .64}
	for i, y := range rows {
		sy := int(y * float64(w))
		sx := int(starts[i] * float64(w))
		line(hi, sx, sy, int(.84*float64(w)), sy, int(.044*float64(w)), ink)
		circleFill(hi, sx-int(.04*float64(w)), sy, int(.023*float64(w)), teal)
	}
	return downsample(hi, size)
}

func downsample(src *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sx := src.Bounds().Dx() / size
	sy := src.Bounds().Dy() / size
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var rr, gg, bb, aa uint32
			for yy := 0; yy < sy; yy++ {
				for xx := 0; xx < sx; xx++ {
					r, g, b, a := src.At(x*sx+xx, y*sy+yy).RGBA()
					rr += r
					gg += g
					bb += b
					aa += a
				}
			}
			n := uint32(sx * sy)
			dst.SetRGBA(x, y, color.RGBA{uint8((rr / n) >> 8), uint8((gg / n) >> 8), uint8((bb / n) >> 8), uint8((aa / n) >> 8)})
		}
	}
	return dst
}

func roundedRect(img *image.RGBA, x0, y0, x1, y1, r int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			dx := max(max(x0+r-x, 0), x-(x1-r-1))
			dy := max(max(y0+r-y, 0), y-(y1-r-1))
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
func circleFill(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	rr := r * r
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= rr {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
func circleStrokeGradient(img *image.RGBA, cx, cy, r, sw int, a, b color.RGBA) {
	outer := r * r
	inner := (r - sw) * (r - sw)
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			d := dx*dx + dy*dy
			if d <= outer && d >= inner {
				t := float64(y-(cy-r)) / float64(2*r)
				img.SetRGBA(x, y, blend(a, b, t))
			}
		}
	}
}
func line(img *image.RGBA, x0, y0, x1, y1, width int, c color.RGBA) {
	dx, dy := float64(x1-x0), float64(y1-y0)
	steps := int(math.Max(math.Abs(dx), math.Abs(dy)))
	if steps < 1 {
		steps = 1
	}
	rad := width / 2
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		circleFill(img, int(float64(x0)+dx*t), int(float64(y0)+dy*t), rad, c)
	}
}
func blend(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{uint8(float64(a.R)*(1-t) + float64(b.R)*t), uint8(float64(a.G)*(1-t) + float64(b.G)*t), uint8(float64(a.B)*(1-t) + float64(b.B)*t), 255}
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

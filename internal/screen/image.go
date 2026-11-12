// Package screen prepares user images for the dock without modifying originals.
package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"math"
)

const MaxInputBytes = 32 * 1024 * 1024
const maxPixels = 20_000_000

// Prepare detects PNG/JPEG content, fits the complete image onto a black 320x240 canvas
// and encodes a JPEG within the dock's 65,520-byte limit. Alpha uses black.
// JPEG EXIF orientation is not applied; callers should provide upright images.
func Prepare(data []byte) ([]byte, error) {
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("input image exceeds 32 MiB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot read image: %w", err)
	}
	if format != "jpeg" && format != "png" {
		return nil, fmt.Errorf("unsupported image format %q; use PNG or JPEG", format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, fmt.Errorf("image exceeds 20 megapixel decode limit")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cannot decode image: %w", err)
	}
	dst := resizeFit(src)
	for quality := 95; quality >= 5; quality -= 5 {
		var out bytes.Buffer
		if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		if out.Len() <= 65520 {
			return out.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("cannot compress image within 65,520 bytes")
}
func resizeFit(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	scale := math.Min(320/w, 240/h)
	fitW, fitH := max(1, int(math.Round(w*scale))), max(1, int(math.Round(h*scale)))
	left, top := (320-fitW)/2, (240-fitH)/2
	dst := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := range 240 {
		for x := range 320 {
			dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	for y := range fitH {
		for x := range fitW {
			sx := math.Max(0, math.Min(w-1, (float64(x)+0.5)*w/float64(fitW)-0.5))
			sy := math.Max(0, math.Min(h-1, (float64(y)+0.5)*h/float64(fitH)-0.5))
			x0, y0 := int(sx), int(sy)
			x1, y1 := min(x0+1, bounds.Dx()-1), min(y0+1, bounds.Dy()-1)
			fx, fy := sx-float64(x0), sy-float64(y0)
			points := [4]image.Point{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}
			weights := [4]float64{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy}
			var channels [3]float64
			for i, p := range points {
				r, g, b, _ := src.At(bounds.Min.X+p.X, bounds.Min.Y+p.Y).RGBA()
				channels[0] += float64(r) * weights[i]
				channels[1] += float64(g) * weights[i]
				channels[2] += float64(b) * weights[i]
			}
			dst.SetRGBA(left+x, top+y, color.RGBA{uint8(channels[0]/257 + 0.5), uint8(channels[1]/257 + 0.5), uint8(channels[2]/257 + 0.5), 255})
		}
	}
	return dst
}

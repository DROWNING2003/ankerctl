package screen

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"strings"
	"testing"
)

func TestPreparePNGAspectFit(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 640, 240))
	for y := range 240 {
		for x := range 640 {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 160 && x < 480 {
				c = color.RGBA{0, 255, 0, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, src); err != nil {
		t.Fatal(err)
	}
	out, err := Prepare(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 320 || decoded.Bounds().Dy() != 240 || len(out) > 65520 {
		t.Fatalf("bounds=%v bytes=%d", decoded.Bounds(), len(out))
	}
	r, g, b, _ := decoded.At(5, 5).RGBA()
	if r+g+b > 1000 {
		t.Fatal("expected black letterbox")
	}
	r, g, _, _ = decoded.At(5, 120).RGBA()
	if r < 50000 || g > 5000 {
		t.Fatal("left edge was cropped")
	}
	r, g, _, _ = decoded.At(160, 120).RGBA()
	if g < 50000 || r > 5000 {
		t.Fatal("center should remain green")
	}
	r, g, _, _ = decoded.At(315, 120).RGBA()
	if r < 50000 || g > 5000 {
		t.Fatal("right edge was cropped")
	}
}
func TestPrepareTransparencyAndCompression(t *testing.T) {
	transparent := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	var input bytes.Buffer
	_ = png.Encode(&input, transparent)
	out, err := Prepare(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := jpeg.Decode(bytes.NewReader(out))
	r, g, b, _ := decoded.At(100, 100).RGBA()
	if r+g+b > 1000 {
		t.Fatal("transparent pixels must composite on black")
	}
	noise := image.NewRGBA(image.Rect(0, 0, 320, 240))
	rng := rand.New(rand.NewSource(1))
	for y := range 240 {
		for x := range 320 {
			noise.SetRGBA(x, y, color.RGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), 255})
		}
	}
	input.Reset()
	_ = png.Encode(&input, noise)
	out, err = Prepare(input.Bytes())
	if err != nil || len(out) > 65520 {
		t.Fatalf("bytes=%d err=%v", len(out), err)
	}
}
func TestPreparePortraitKeepsEdges(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 200))
	for y := range 200 {
		for x := range 100 {
			src.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	var input bytes.Buffer
	_ = jpeg.Encode(&input, src, nil)
	out, err := Prepare(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []image.Point{{160, 1}, {160, 238}} {
		r, g, b, _ := decoded.At(p.X, p.Y).RGBA()
		if r+g+b < 180000 {
			t.Fatal("portrait edge cropped")
		}
	}
	r, g, b, _ := decoded.At(5, 120).RGBA()
	if r+g+b > 1000 {
		t.Fatal("missing pillarbox")
	}
}
func TestPrepareInputLimits(t *testing.T) {
	if _, err := Prepare(make([]byte, MaxInputBytes+1)); err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Fatalf("size limit: %v", err)
	}
	var input bytes.Buffer
	_ = png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	header := input.Bytes()
	binary.BigEndian.PutUint32(header[16:20], 5000)
	binary.BigEndian.PutUint32(header[20:24], 5000)
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	if _, err := Prepare(header); err == nil || !strings.Contains(err.Error(), "megapixel") {
		t.Fatalf("pixel limit: %v", err)
	}
}
func TestPrepareRejectsInvalid(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not an image"), {0xff, 0xd8}} {
		if _, err := Prepare(data); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
}

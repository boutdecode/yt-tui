package thumb

import (
	"image"
	"image/color"
	"testing"
)

func solid(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestDominantRed(t *testing.T) {
	c, ok := Dominant(solid(color.RGBA{200, 30, 30, 255}))
	if !ok {
		t.Fatal("rouge uni : ok attendu")
	}
	h, _, _ := RGBToHSL(c.Primary)
	if h > 15 && h < 345 {
		t.Errorf("teinte primaire = %.0f°, attendu ≈ rouge", h)
	}
	h2, _, _ := RGBToHSL(c.Secondary)
	if d := h2 - h; d > -30 && d < 30 {
		t.Errorf("la seconde teinte (%.0f°) doit s'écarter de la première (%.0f°)", h2, h)
	}
}

func TestDominantNeutral(t *testing.T) {
	for _, c := range []color.RGBA{{0, 0, 0, 255}, {255, 255, 255, 255}, {128, 128, 128, 255}} {
		if _, ok := Dominant(solid(c)); ok {
			t.Errorf("%v : image terne, ok=false attendu", c)
		}
	}
}

func TestDominantTwoZones(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			if x < 40 {
				img.Set(x, y, color.RGBA{30, 60, 220, 255})
			} else {
				img.Set(x, y, color.RGBA{230, 200, 20, 255})
			}
		}
	}
	c, ok := Dominant(img)
	if !ok {
		t.Fatal("ok attendu")
	}
	if c.Primary.B < c.Primary.R {
		t.Errorf("primaire = %v, attendu bleu", c.Primary)
	}
	if c.Secondary.R < c.Secondary.B {
		t.Errorf("secondaire = %v, attendu jaune", c.Secondary)
	}
}

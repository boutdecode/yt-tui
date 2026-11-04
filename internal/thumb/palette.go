package thumb

import (
	"context"
	"image"
	"image/color"
	"math"
)

// Colors résume les couleurs marquantes d'une image : la teinte dominante et une
// seconde teinte, éloignée de la première.
type Colors struct {
	Primary, Secondary color.RGBA
}

const hueBuckets = 16

// Dominant extrait les couleurs dominantes de img. ok vaut false si l'image est
// (quasi) en niveaux de gris : aucune teinte n'y ressort.
func Dominant(img image.Image) (c Colors, ok bool) {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return c, false
	}
	const n = 32
	var weight [hueBuckets]float64
	var sumR, sumG, sumB [hueBuckets]float64
	var total float64
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			x := b.Min.X + (2*i+1)*b.Dx()/(2*n)
			y := b.Min.Y + (2*j+1)*b.Dy()/(2*n)
			r, g, bl, _ := img.At(x, y).RGBA()
			rf, gf, bf := float64(r)/65535, float64(g)/65535, float64(bl)/65535
			h, s, l := rgbToHSL(rf, gf, bf)
			// Les pixels ternes (gris, très sombres ou très clairs) ne portent pas de teinte.
			w := s * (1 - math.Abs(2*l-1))
			if w < 0.04 {
				continue
			}
			k := int(h/360*hueBuckets) % hueBuckets
			weight[k] += w
			sumR[k] += rf * w
			sumG[k] += gf * w
			sumB[k] += bf * w
			total += w
		}
	}
	// Moins de ~4 % de pixels vraiment colorés : image grise.
	if total < 0.04*n*n*0.5 {
		return c, false
	}
	best := argmax(weight[:], -1)
	c.Primary = bucketColor(sumR[best], sumG[best], sumB[best], weight[best])
	// Seconde teinte : meilleur seau à au moins ~40° de la première, assez présent.
	second, secondW := -1, 0.0
	for k := 0; k < hueBuckets; k++ {
		d := math.Abs(float64(k - best))
		d = math.Min(d, hueBuckets-d) * 360 / hueBuckets
		if d >= 40 && weight[k] > secondW && weight[k] >= 0.15*weight[best] {
			second, secondW = k, weight[k]
		}
	}
	if second >= 0 {
		c.Secondary = bucketColor(sumR[second], sumG[second], sumB[second], weight[second])
	} else {
		h, s, l := rgbToHSL(float64(c.Primary.R)/255, float64(c.Primary.G)/255, float64(c.Primary.B)/255)
		c.Secondary = hslToRGBA(math.Mod(h+40, 360), s, l)
	}
	return c, true
}

func argmax(v []float64, skip int) int {
	best := -1
	for i, x := range v {
		if i != skip && (best < 0 || x > v[best]) {
			best = i
		}
	}
	return best
}

func bucketColor(r, g, b, w float64) color.RGBA {
	return color.RGBA{uint8(r / w * 255), uint8(g / w * 255), uint8(b / w * 255), 255}
}

// Palette calcule les couleurs dominantes de la miniature de id (cache disque puis réseau).
func (s *Store) Palette(ctx context.Context, id string) (Colors, bool, error) {
	s.mu.Lock()
	if p, ok := s.pal[id]; ok {
		s.mu.Unlock()
		return p.c, p.ok, nil
	}
	s.mu.Unlock()
	img, err := s.image(ctx, id)
	if err != nil {
		return Colors{}, false, err
	}
	c, ok := Dominant(img)
	s.mu.Lock()
	s.pal[id] = palEntry{c, ok}
	s.mu.Unlock()
	return c, ok, nil
}

type palEntry struct {
	c  Colors
	ok bool
}

// rgbToHSL : r,g,b ∈ [0,1] → h ∈ [0,360), s,l ∈ [0,1].
func rgbToHSL(r, g, b float64) (h, s, l float64) {
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	d := mx - mn
	if d == 0 {
		return 0, 0, l
	}
	s = d / (1 - math.Abs(2*l-1))
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return
}

func hslToRGBA(h, s, l float64) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{uint8(math.Round((r + m) * 255)), uint8(math.Round((g + m) * 255)), uint8(math.Round((b + m) * 255)), 255}
}

// RGBToHSL et HSLToRGBA exposent la conversion de couleurs à l'interface (thème).
func RGBToHSL(c color.RGBA) (h, s, l float64) {
	return rgbToHSL(float64(c.R)/255, float64(c.G)/255, float64(c.B)/255)
}

func HSLToRGBA(h, s, l float64) color.RGBA { return hslToRGBA(h, s, l) }

package go2dpf

import (
	"image"
	"image/color"
)

// ColorRGB565 represents a 16-bit 565 color.
type ColorRGB565 struct {
	C uint16
}

func (c ColorRGB565) RGBA() (r, g, b, a uint32) {
	r = uint32((c.C >> 11 & 0x1f) << 3)
	r |= r << 8
	g = uint32((c.C >> 5 & 0x3f) << 2)
	g |= g << 8
	b = uint32((c.C & 0x1f) << 3)
	b |= b << 8
	a = 0xffff // No alpha color
	return
}

func rgb565Model(c color.Color) color.Color {
	if _, ok := c.(ColorRGB565); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	return ColorRGB565{
		uint16((r & 0xF800) >> 0) |
		uint16((g & 0xFC00) >> 5) |
		uint16((b & 0xFC00) >> (5+6))}
}

var RGB565Model color.Model = color.ModelFunc(rgb565Model)

// RGB565 is an in-memory image whose At method returns ColorRGB565 values.
type ImageRGB565 struct {
	// Pix holds the image's pixels, as alpha values in big-endian format. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*2].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

func (p *ImageRGB565) ColorModel() color.Model { return RGB565Model }

func (p *ImageRGB565) Bounds() image.Rectangle { return p.Rect }

func (p *ImageRGB565) At(x, y int) color.Color {
	return p.RGB565At(x, y)
}

func (p *ImageRGB565) RGB565At(x, y int) ColorRGB565 {
	if !(image.Point{x, y}.In(p.Rect)) {
		return ColorRGB565{}
	}
	i := p.PixOffset(x, y)
	return ColorRGB565{uint16(p.Pix[i+0])<<8 | uint16(p.Pix[i+1])}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *ImageRGB565) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*2
}

func (p *ImageRGB565) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := RGB565Model.Convert(c).(ColorRGB565)
	p.Pix[i+0] = uint8(c1.C >> 8)
	p.Pix[i+1] = uint8(c1.C)
}

func (p *ImageRGB565) SetRGB565(x, y int, c ColorRGB565) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i+0] = uint8(c.C >> 8)
	p.Pix[i+1] = uint8(c.C)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *ImageRGB565) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &ImageRGB565{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &ImageRGB565{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *ImageRGB565) Opaque() bool {
	return true
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 0, p.Rect.Dx()*2
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i += 2 {
			if p.Pix[i+0] != 0xff || p.Pix[i+1] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewRGB565 returns a new RGB565 image with the given bounds.
func NewRGB565(r image.Rectangle) *ImageRGB565 {
	w, h := r.Dx(), r.Dy()
	pix := make([]uint8, 2*w*h)
	return &ImageRGB565{pix, 2 * w, r}
}

// NewRGB565 returns RGB565 copy of image
func NewRGB565Image(src image.Image) *ImageRGB565 {
	r := src.Bounds()
	dst := NewRGB565(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}


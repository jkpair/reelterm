package thumb

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mattn/go-sixel"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Rendered is a thumbnail prepared for one cell size.
type Rendered struct {
	Cols, Rows int
	// Text is in-grid output (half blocks, chafa). Empty for overlays.
	Text string
	// Data is the escape sequence payload for overlay protocols.
	Data []byte
	// OffX centres the image horizontally within the reserved cells.
	OffX int
}

func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// fit scales src to fit within maxW x maxH pixels, keeping aspect ratio.
func fit(src image.Image, maxW, maxH int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	w, h := maxW, sh*maxW/sw
	if h > maxH {
		h, w = maxH, sw*maxH/sh
	}
	w, h = max(w, 1), max(h, 1)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

// cropLetterbox removes the black bars YouTube bakes into 4:3 hqdefault thumbnails.
func cropLetterbox(img image.Image) image.Image {
	b := img.Bounds()
	if b.Dx()*3 != b.Dy()*4 {
		return img
	}
	h := b.Dx() * 9 / 16
	top := (b.Dy() - h) / 2
	dark := func(y int) bool {
		for x := b.Min.X; x < b.Max.X; x += max(b.Dx()/16, 1) {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 > 24 || g>>8 > 24 || bl>>8 > 24 {
				return false
			}
		}
		return true
	}
	if !dark(b.Min.Y+top/2) || !dark(b.Max.Y-1-top/2) {
		return img
	}
	if s, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return s.SubImage(image.Rect(b.Min.X, b.Min.Y+top, b.Max.X, b.Min.Y+top+h))
	}
	return img
}

// render prepares path for a cols x rows cell area.
func render(p Protocol, path string, cols, rows, cw, ch int) (*Rendered, error) {
	if p == Chafa {
		return renderChafa(path, cols, rows)
	}
	src, err := decode(path)
	if err != nil {
		return nil, err
	}
	src = cropLetterbox(src)
	r := &Rendered{Cols: cols, Rows: rows}
	switch p {
	case HalfBlock:
		img := fit(src, cols, rows*2)
		r.OffX = (cols - img.Bounds().Dx()) / 2
		r.Text = halfBlocks(img, r.OffX, cols, rows)
		return r, nil
	}
	img := fit(src, cols*cw, rows*ch)
	imgCols := (img.Bounds().Dx() + cw - 1) / cw
	r.OffX = max((cols-imgCols)/2, 0)
	var buf bytes.Buffer
	switch p {
	case Sixel:
		if err := sixel.NewEncoder(&buf).Encode(img); err != nil {
			return nil, err
		}
	case Kitty:
		kittyTransmit(&buf, img)
	case ITerm:
		var pngBuf bytes.Buffer
		if err := png.Encode(&pngBuf, img); err != nil {
			return nil, err
		}
		imgRows := (img.Bounds().Dy() + ch - 1) / ch
		fmt.Fprintf(&buf, "\x1b]1337;File=inline=1;size=%d;width=%d;height=%d;preserveAspectRatio=1:%s\a",
			pngBuf.Len(), imgCols, imgRows, base64.StdEncoding.EncodeToString(pngBuf.Bytes()))
	}
	r.Data = buf.Bytes()
	return r, nil
}

// kittyTransmit writes a transmit-only command (a=t) for img,
// zlib-compressed RGBA in 4096-byte chunks. The image id is filled in at
// draw time via the "%ID%" placeholder.
func kittyTransmit(buf *bytes.Buffer, img *image.RGBA) {
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	_, _ = zw.Write(img.Pix)
	_ = zw.Close()
	enc := base64.StdEncoding.EncodeToString(z.Bytes())
	b := img.Bounds()
	first := true
	for len(enc) > 0 {
		n := min(4096, len(enc))
		chunk := enc[:n]
		enc = enc[n:]
		more := 0
		if len(enc) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(buf, "\x1b_Ga=t,f=32,o=z,s=%d,v=%d,i=%%ID%%,q=2,m=%d;%s\x1b\\", b.Dx(), b.Dy(), more, chunk)
			first = false
		} else {
			fmt.Fprintf(buf, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
}

func halfBlocks(img *image.RGBA, offX, cols, rows int) string {
	var b strings.Builder
	bw, bh := img.Bounds().Dx(), img.Bounds().Dy()
	at := func(x, y int) color.RGBA {
		if x < 0 || x >= bw || y >= bh {
			return color.RGBA{}
		}
		return img.RGBAAt(x, y)
	}
	padTop := (rows*2 - bh) / 4
	for row := range rows {
		y := (row - padTop) * 2
		for col := range cols {
			x := col - offX
			if y < 0 || x < 0 || x >= bw || y >= bh {
				b.WriteByte(' ')
				continue
			}
			t, u := at(x, y), at(x, y+1)
			if y+1 >= bh {
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm▀\x1b[0m", t.R, t.G, t.B)
				continue
			}
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀\x1b[0m", t.R, t.G, t.B, u.R, u.G, u.B)
		}
		if row < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderChafa(path string, cols, rows int) (*Rendered, error) {
	out, err := exec.Command("chafa", "-f", "symbols", "--animate=off", "--polite=on",
		"-s", strconv.Itoa(cols)+"x"+strconv.Itoa(rows), path).Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}
	return &Rendered{Cols: cols, Rows: rows, Text: strings.Join(lines, "\n")}, nil
}

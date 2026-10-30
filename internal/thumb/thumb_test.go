package thumb

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func writeJPEG(t *testing.T, path string, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRenderers(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.jpg")
	writeJPEG(t, p, 320, 180)
	for _, proto := range []Protocol{HalfBlock, Kitty, Sixel, ITerm} {
		r, err := render(proto, p, 40, 11, 10, 20)
		if err != nil {
			t.Fatalf("%s: %v", proto, err)
		}
		switch proto {
		case HalfBlock:
			if n := strings.Count(r.Text, "\n"); n != 10 {
				t.Errorf("halfblock rows = %d", n+1)
			}
		case Kitty:
			if !bytes.HasPrefix(r.Data, []byte("\x1b_Ga=t,f=32,o=z")) || !bytes.Contains(r.Data, []byte("i=%ID%")) {
				t.Errorf("kitty payload: %q", r.Data[:40])
			}
		case Sixel:
			if !bytes.HasPrefix(r.Data, []byte("\x1bP")) {
				t.Errorf("sixel payload: %q", r.Data[:10])
			}
		case ITerm:
			if !bytes.HasPrefix(r.Data, []byte("\x1b]1337;File=inline=1")) {
				t.Errorf("iterm payload: %q", r.Data[:30])
			}
		}
	}
}

func TestLetterboxCrop(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 480, 360))
	for y := 45; y < 315; y++ {
		for x := range 480 {
			img.Set(x, y, color.White)
		}
	}
	got := cropLetterbox(img).Bounds()
	if got.Dx() != 480 || got.Dy() != 270 {
		t.Fatalf("crop = %v", got)
	}
}

func TestOverlayDrawSequence(t *testing.T) {
	dir := t.TempDir()
	writeJPEG(t, filepath.Join(dir, "k"), 320, 180)
	var out bytes.Buffer
	sent := make(chan tea.Msg, 4)
	m := New(Kitty, func(k string) string { return filepath.Join(dir, k) }, &out, 1)
	defer m.Close()
	m.Send = func(msg tea.Msg) { sent <- msg }
	m.cw, m.ch = 10, 20

	// First frame kicks off rendering; nothing is placed yet.
	m.BeginFrame()
	if m.Place("k", 1, 5, 20, 6) {
		t.Fatal("placed before render finished")
	}
	m.EndFrame()
	select {
	case <-m.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("render never finished")
	}

	m.BeginFrame()
	if !m.Place("k", 1, 5, 20, 6) {
		t.Fatal("not placed after render")
	}
	m.EndFrame()
	var msg tea.Msg
	select {
	case msg = <-sent:
	case <-time.After(time.Second):
		t.Fatal("no draw scheduled")
	}
	m.HandleDraw(msg)
	s := out.String()
	if !strings.HasPrefix(s, "\x1b7") || !strings.HasSuffix(s, "\x1b8") {
		t.Errorf("draw not wrapped in save/restore cursor")
	}
	if !strings.Contains(s, "\x1b[6;2H") || !strings.Contains(s, "i=4243,") {
		t.Errorf("unexpected draw output prefix: %q", s[:min(len(s), 120)])
	}

	// A frame without Place removes the image.
	out.Reset()
	m.BeginFrame()
	m.EndFrame()
	m.HandleDraw(<-sent)
	if !strings.Contains(out.String(), "a=d,d=i,i=4243") {
		t.Errorf("kitty placement not deleted: %q", out.String())
	}
	// Removing must not touch cells: the next screen is already drawn there.
	if strings.Contains(out.String(), "H ") {
		t.Errorf("removal wrote over the screen: %q", out.String())
	}
}

// Sixel images are part of the text grid; leaving the screen must not
// blank cells (the renderer has redrawn them), only drawing does.
func TestSixelLeaveDoesNotBlank(t *testing.T) {
	dir := t.TempDir()
	writeJPEG(t, filepath.Join(dir, "k"), 320, 180)
	var out bytes.Buffer
	sent := make(chan tea.Msg, 8)
	m := New(Sixel, func(k string) string { return filepath.Join(dir, k) }, &out, 1)
	defer m.Close()
	m.Send = func(msg tea.Msg) { sent <- msg }
	m.cw, m.ch = 10, 20
	m.BeginFrame()
	m.Place("k", 1, 5, 20, 6)
	m.EndFrame()
	<-m.ready
	m.BeginFrame()
	m.Place("k", 1, 5, 20, 6)
	m.EndFrame()
	m.HandleDraw(<-sent)
	if !strings.Contains(out.String(), "\x1bP") {
		t.Fatal("sixel not drawn")
	}
	out.Reset()
	m.BeginFrame()
	m.EndFrame()
	m.HandleDraw(<-sent)
	if out.Len() > len("\x1b7\x1b8") {
		t.Errorf("leaving wrote to the screen: %q", out.String())
	}
}

// Frames keep arriving while a draw is pending (each draw tick causes one).
// The draw must still happen, and showing the same image again must place
// it by id without re-transmitting it.
func TestOverlayNotStarvedAndReused(t *testing.T) {
	dir := t.TempDir()
	writeJPEG(t, filepath.Join(dir, "k"), 320, 180)
	var out bytes.Buffer
	sent := make(chan tea.Msg, 64)
	m := New(Kitty, func(k string) string { return filepath.Join(dir, k) }, &out, 1)
	defer m.Close()
	m.Send = func(msg tea.Msg) { sent <- msg }
	m.cw, m.ch = 10, 20
	frame := func(place bool) {
		m.BeginFrame()
		if place {
			m.Place("k", 1, 5, 20, 6)
		}
		m.EndFrame()
	}
	frame(true)
	<-m.ready
	for range 50 {
		frame(true)
	}
	if n := len(sent); n > 1 {
		t.Fatalf("scheduled %d draws for one change", n)
	}
	m.HandleDraw(<-sent)
	if !strings.Contains(out.String(), "a=t,") || !strings.Contains(out.String(), "a=p,i=4243") {
		t.Fatalf("first draw should transmit and place: %q", out.String()[:min(out.Len(), 80)])
	}
	m.Invalidate() // pushing a screen sends a size refresh
	frame(false)   // leave the screen
	m.HandleDraw(<-sent)
	if !strings.Contains(out.String(), "a=d,d=i,i=4243") {
		t.Fatalf("image not removed after leaving the screen: %q", out.String())
	}
	out.Reset()
	frame(true) // come back
	m.HandleDraw(<-sent)
	if !strings.Contains(out.String(), "a=t,") || !strings.Contains(out.String(), "a=p,i=4244") {
		t.Fatalf("after a screen clear the image must be re-sent: %q", out.String()[:min(out.Len(), 80)])
	}

	// Moving between images without a clear reuses the stored data.
	writeJPEG(t, filepath.Join(dir, "k2"), 320, 180)
	show := func(key string) {
		m.BeginFrame()
		m.Place(key, 1, 5, 20, 6)
		m.EndFrame()
		time.Sleep(80 * time.Millisecond)
		for len(sent) > 0 {
			m.HandleDraw(<-sent)
		}
	}
	show("k2") // starts rendering k2
	<-m.ready
	show("k2")
	out.Reset()
	show("k")
	if strings.Contains(out.String(), "a=t,") || !strings.Contains(out.String(), "a=p,i=4244") {
		t.Fatalf("returning to a stored image should only place it: %q", out.String())
	}
}

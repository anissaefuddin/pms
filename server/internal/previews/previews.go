// Package previews renders the proofing preview of one file version (FIL-02, D-80): a bounded PNG per page,
// produced by re-encoding, so the viewer never serves the upload itself.
package previews

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

const (
	MaxSourceBytes = 50 << 20 // larger files are skipped (status "skipped")
	MaxPixels      = 50_000_000
	MaxEdge        = 1600
	MaxPDFPages    = 20
)

type Args struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	Version      int       `json:"version"`
}

func (Args) Kind() string { return "attachment_preview" }

type Page struct {
	Page   int    `json:"page"`
	Key    string `json:"key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Worker struct {
	river.WorkerDefaults[Args]
	Pool   *pgxpool.Pool
	Config config.Config
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	return w.Generate(ctx, job.Args.AttachmentID, job.Args.Version)
}

// Generate reads the source from the backend it was recorded in and stores the pages in the configured one.
func (w *Worker) Generate(ctx context.Context, id uuid.UUID, version int) error {
	source := func(backend string) (storage.Storage, error) { return storage.For(w.Config, backend) }
	return generate(ctx, w.Pool, source, storage.FromConfig(w.Config), id, version)
}

// Generate fills attachment_previews for one version. Unsupported or oversized input is "skipped", a broken file "failed";
// neither is retried. Only database or storage errors are returned (River retries those).
func Generate(ctx context.Context, pool *pgxpool.Pool, store storage.Storage, id uuid.UUID, version int) error {
	return generate(ctx, pool, func(string) (storage.Storage, error) { return store, nil }, store, id, version)
}

func generate(ctx context.Context, pool *pgxpool.Pool, sourceOf func(backend string) (storage.Storage, error), store storage.Storage, id uuid.UUID, version int) error {
	var media, key, backend string
	var size int64
	err := pool.QueryRow(ctx, "SELECT content_type,storage_key,size_bytes,coalesce(storage_backend,'') FROM attachment_versions WHERE attachment_id=$1 AND version=$2", id, version).Scan(&media, &key, &size, &backend)
	if err != nil {
		return err
	}
	source, err := sourceOf(backend)
	if err != nil {
		return err
	}
	pages, status, note := []Page{}, "ready", ""
	switch {
	case size > MaxSourceBytes:
		status, note = "skipped", "file is larger than the preview limit"
	case media == "application/pdf":
		pages, status, note = renderPDF(ctx, source, store, key, size)
	case media == "image/png" || media == "image/jpeg" || media == "image/gif":
		var p Page
		if p, status, note = renderImage(source, store, key); status == "ready" {
			pages = []Page{p}
		}
	default:
		status, note = "skipped", "no preview for this file type"
	}
	if pages == nil {
		pages = []Page{} // a skipped render must store [], not JSON null (jsonb_array_elements fails on null)
	}
	raw, _ := json.Marshal(pages)
	_, err = pool.Exec(ctx, `INSERT INTO attachment_previews(attachment_id,version,status,pages,note) VALUES($1,$2,$3,$4,$5)
ON CONFLICT (attachment_id,version) DO UPDATE SET status=$3,pages=$4,note=$5,updated_at=now()`, id, version, status, raw, note)
	return err
}

func renderImage(source, store storage.Storage, key string) (Page, string, string) {
	f, err := source.Open(key)
	if err != nil {
		return Page{}, "failed", "file cannot be read"
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > MaxPixels {
		return Page{}, "skipped", "image is invalid or too large to preview"
	}
	g, err := source.Open(key)
	if err != nil {
		return Page{}, "failed", "file cannot be read"
	}
	defer g.Close()
	return storePNG(store, g, 1)
}

// storePNG decodes, scales to MaxEdge and stores a PNG; re-encoding drops metadata and anything embedded in the upload.
func storePNG(store storage.Storage, src io.Reader, page int) (Page, string, string) {
	var img image.Image
	var err error
	// DecodeConfig above already registered the formats; PNG/JPEG/GIF are all handled by image.Decode.
	if img, _, err = image.Decode(src); err != nil {
		return Page{}, "failed", "image cannot be decoded"
	}
	img = fit(img)
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return Page{}, "failed", "preview cannot be encoded"
	}
	file, err := store.Put(&out, MaxSourceBytes)
	if err != nil {
		return Page{}, "failed", "preview cannot be stored"
	}
	b := img.Bounds()
	return Page{Page: page, Key: file.Key, Width: b.Dx(), Height: b.Dy()}, "ready", ""
}

// fit scales down (nearest neighbour; previews only) so the longest edge is at most MaxEdge.
func fit(src image.Image) image.Image {
	b := src.Bounds()
	long := max(b.Dx(), b.Dy())
	if long <= MaxEdge {
		return src
	}
	w, h := max(1, b.Dx()*MaxEdge/long), max(1, b.Dy()*MaxEdge/long)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, color.NRGBAModel.Convert(src.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h)))
		}
	}
	return dst
}

var _ = []any{jpeg.Decode, gif.Decode} // register the decoders

// renderPDF needs poppler's pdftoppm; without it PDFs are skipped. Pages beyond MaxPDFPages are not rendered.
func renderPDF(ctx context.Context, source, store storage.Storage, key string, size int64) ([]Page, string, string) {
	bin, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, "skipped", "pdftoppm is not installed"
	}
	dir, err := os.MkdirTemp("", "pms-preview-")
	if err != nil {
		return nil, "failed", "no temporary directory"
	}
	defer os.RemoveAll(dir)
	src, err := source.Open(key)
	if err != nil {
		return nil, "failed", "file cannot be read"
	}
	defer src.Close()
	in, err := os.Create(filepath.Join(dir, "in.pdf"))
	if err != nil {
		return nil, "failed", "no temporary file"
	}
	if _, err = io.Copy(in, io.NewSectionReader(src, 0, size)); err != nil {
		return nil, "failed", "no temporary file"
	}
	in.Close()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-png", "-scale-to", fmt.Sprint(MaxEdge), "-f", "1", "-l", fmt.Sprint(MaxPDFPages), in.Name(), filepath.Join(dir, "p"))
	if err := cmd.Run(); err != nil {
		return nil, "failed", "pdf cannot be rendered"
	}
	files, _ := filepath.Glob(filepath.Join(dir, "p-*.png"))
	if len(files) == 0 {
		return nil, "failed", "pdf has no pages"
	}
	pages := []Page{}
	for i, name := range files { // pdftoppm zero-pads the page number, so the glob order is page order
		f, err := os.Open(name)
		if err != nil {
			return nil, "failed", "page cannot be read"
		}
		p, status, note := storePNG(store, f, i+1)
		f.Close()
		if status != "ready" {
			return nil, status, note
		}
		pages = append(pages, p)
	}
	return pages, "ready", ""
}

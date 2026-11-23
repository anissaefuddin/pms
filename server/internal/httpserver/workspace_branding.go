package httpserver

import (
	"errors"
	"io"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"
)

var workspaceAssetKinds = []string{"icon", "logo_light", "logo_dark"}

type publicWorkspaceBranding struct {
	Name         string `json:"name"`
	BrandColor   string `json:"brand_color"`
	IconVersion  int64  `json:"icon_image_version"`
	LightVersion int64  `json:"logo_light_version"`
	DarkVersion  int64  `json:"logo_dark_version"`
}

// workspaceBranding exposes only public visual identity, including to signed-out pages.
func (s *Server) workspaceBranding(w http.ResponseWriter, r *http.Request) {
	var brand publicWorkspaceBranding
	err := s.pool.QueryRow(r.Context(), `SELECT name,COALESCE(NULLIF(settings->>'brand_color',''),'#2563EB'),
COALESCE((SELECT (EXTRACT(EPOCH FROM updated_at)*1000)::bigint FROM workspace_brand_assets WHERE kind='icon'),0),
COALESCE((SELECT (EXTRACT(EPOCH FROM updated_at)*1000)::bigint FROM workspace_brand_assets WHERE kind='logo_light'),0),
COALESCE((SELECT (EXTRACT(EPOCH FROM updated_at)*1000)::bigint FROM workspace_brand_assets WHERE kind='logo_dark'),0)
FROM account LIMIT 1`).Scan(&brand.Name, &brand.BrandColor, &brand.IconVersion, &brand.LightVersion, &brand.DarkVersion)
	if err != nil {
		writeError(w, 500, "could not load workspace branding")
		return
	}
	writeJSON(w, 200, brand)
}

func workspaceAssetKind(r *http.Request) (string, bool) {
	kind := r.PathValue("asset")
	return kind, slices.Contains(workspaceAssetKinds, kind)
}

func (s *Server) workspaceBrandAsset(w http.ResponseWriter, r *http.Request) {
	kind, ok := workspaceAssetKind(r)
	if !ok {
		writeError(w, 404, "unknown branding asset")
		return
	}
	var contentType string
	var data []byte
	err := s.pool.QueryRow(r.Context(), "SELECT content_type,data FROM workspace_brand_assets WHERE kind=$1", kind).Scan(&contentType, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "branding asset not found")
		return
	}
	if err != nil {
		writeError(w, 500, "could not load branding asset")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func (s *Server) saveWorkspaceBrandAsset(w http.ResponseWriter, r *http.Request) {
	kind, ok := workspaceAssetKind(r)
	if !ok {
		writeError(w, 404, "unknown branding asset")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, avatarMaxBytes+1))
	if err != nil || len(data) > avatarMaxBytes {
		writeError(w, 413, "branding image must be 2 MB or smaller")
		return
	}
	contentType := http.DetectContentType(data)
	if !avatarTypes[contentType] {
		writeError(w, 422, "branding image must be a PNG, JPEG or WebP image")
		return
	}
	if _, err := s.pool.Exec(r.Context(), `INSERT INTO workspace_brand_assets(kind,content_type,data) VALUES($1,$2,$3)
ON CONFLICT (kind) DO UPDATE SET content_type=EXCLUDED.content_type,data=EXCLUDED.data,updated_at=now()`, kind, contentType, data); err != nil {
		writeError(w, 500, "could not save branding asset")
		return
	}
	s.accountInfo(w, r)
}

func (s *Server) removeWorkspaceBrandAsset(w http.ResponseWriter, r *http.Request) {
	kind, ok := workspaceAssetKind(r)
	if !ok {
		writeError(w, 404, "unknown branding asset")
		return
	}
	if _, err := s.pool.Exec(r.Context(), "DELETE FROM workspace_brand_assets WHERE kind=$1", kind); err != nil {
		writeError(w, 500, "could not remove branding asset")
		return
	}
	s.accountInfo(w, r)
}

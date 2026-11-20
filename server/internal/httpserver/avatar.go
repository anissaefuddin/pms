package httpserver

import (
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
)

const avatarMaxBytes = 2 << 20 // matches the CHECK on user_avatars.data

// avatarTypes lists the only photo types. The type comes from the bytes (http.DetectContentType), never from the request header.
var avatarTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

func (s *Server) avatar(w http.ResponseWriter, r *http.Request) {
	var contentType string
	var data []byte
	err := s.pool.QueryRow(r.Context(), "SELECT content_type,data FROM user_avatars WHERE user_id=$1::uuid", r.Context().Value(userKey).(string)).Scan(&contentType, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "no profile photo")
		return
	}
	if err != nil {
		writeError(w, 500, "could not load profile photo")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600") // the URL carries avatar_version, so a new photo gets a new URL
	_, _ = w.Write(data)
}

func (s *Server) saveAvatar(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, avatarMaxBytes+1))
	if err != nil || len(data) > avatarMaxBytes {
		writeError(w, 413, "photo must be 2 MB or smaller")
		return
	}
	contentType := http.DetectContentType(data)
	if !avatarTypes[contentType] {
		writeError(w, 422, "photo must be a PNG, JPEG or WebP image")
		return
	}
	userID := r.Context().Value(userKey).(string)
	if _, err := s.pool.Exec(r.Context(), `INSERT INTO user_avatars(user_id,content_type,data) VALUES($1::uuid,$2,$3)
ON CONFLICT (user_id) DO UPDATE SET content_type=EXCLUDED.content_type,data=EXCLUDED.data,updated_at=now()`, userID, contentType, data); err != nil {
		writeError(w, 500, "could not save profile photo")
		return
	}
	s.writeMe(w, r, userID)
}

func (s *Server) removeAvatar(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userKey).(string)
	if _, err := s.pool.Exec(r.Context(), "DELETE FROM user_avatars WHERE user_id=$1::uuid", userID); err != nil {
		writeError(w, 500, "could not remove profile photo")
		return
	}
	s.writeMe(w, r, userID)
}

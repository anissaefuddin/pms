package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/pydio"
	"github.com/anissaefuddin/pms/server/internal/pydiostore"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
)

// uploadTarget is where one upload goes and which rules it follows.
type uploadTarget struct {
	backend    string
	store      storage.Storage
	max        int64
	put        func(filename string, source io.Reader) (storage.File, error)
	limits     *pydio.UploadLimits // set for Pydio
	attachment uuid.UUID           // set when the path carries the id of a new attachment
}

// localTarget is the backend of PMS_STORAGE. Form submissions always use it: their item does not exist yet.
func (s *Server) localTarget(max int64) uploadTarget {
	store := s.attachmentStore()
	return uploadTarget{backend: storage.Name(s.cfg), store: store, max: max,
		put: func(_ string, source io.Reader) (storage.File, error) { return store.Put(source, max) }}
}

// uploadMax is the size limit of the next upload to an item.
func (s *Server) uploadMax(ctx context.Context) (int64, error) {
	saved, found, err := pydiostore.Load(ctx, s.pool)
	if err != nil || !found || !saved.Enabled {
		return s.attachmentLimit(), err
	}
	limits, err := s.uploadLimits(ctx)
	return int64(limits.MaxMB) << 20, err
}

// uploadTarget picks the target for a file of an item; attachment is nil for a new file. While Pydio storage is on, a problem
// with it fails the upload: a file never lands somewhere else without anyone knowing.
func (s *Server) uploadTarget(ctx context.Context, item, attachment uuid.UUID) (uploadTarget, error) {
	saved, found, err := pydiostore.Load(ctx, s.pool)
	if err != nil {
		return uploadTarget{}, err
	}
	if !found || !saved.Enabled {
		return s.localTarget(s.attachmentLimit()), nil
	}
	client, err := saved.Client(s.cfg)
	if err != nil {
		return uploadTarget{}, err
	}
	limits, err := s.uploadLimits(ctx)
	if err != nil {
		return uploadTarget{}, err
	}
	place, err := pydiostore.FolderOf(ctx, s.pool, saved, item)
	if err != nil {
		return uploadTarget{}, err
	}
	isNew := attachment == uuid.Nil
	if isNew {
		if attachment, err = uuid.NewV7(); err != nil {
			return uploadTarget{}, err
		}
	}
	upload, err := uuid.NewV7()
	if err != nil {
		return uploadTarget{}, err
	}
	max := int64(limits.MaxMB) << 20
	target := uploadTarget{backend: pydiostore.Backend, store: client, max: max, limits: &limits,
		put: func(filename string, source io.Reader) (storage.File, error) {
			return client.PutAt(pydiostore.Key(place.Folder, attachment, upload, filename), source, max)
		}}
	if isNew {
		target.attachment = attachment
	}
	return target, nil
}

func writeStorageUnavailable(w http.ResponseWriter, err error) {
	slog.ErrorContext(logContext(w), "file storage", "error", err)
	writeProblem(w, http.StatusBadGateway, "storage_unavailable", "the file storage did not answer; try again later")
}

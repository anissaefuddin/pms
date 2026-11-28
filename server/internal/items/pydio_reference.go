package items

import "errors"

// ErrPydioFolderUnconfigured is returned for any Pydio folder reference while the instance has no Pydio link configuration.
var ErrPydioFolderUnconfigured = errors.New("Pydio folder references are not configured on this instance")

// PydioFolderCheck validates one canonical Pydio folder reference on every write (B4-02). The server sets it from PMS_PYDIO_LINK_URL and
// PMS_PYDIO_LINK_ROOTS in httpserver.New. Until then it refuses every reference (fail closed). Listing (PMS_PYDIO_LISTING) is separate.
var PydioFolderCheck = func(string) error { return ErrPydioFolderUnconfigured }

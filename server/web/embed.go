package web

import "embed"

// Dist is replaced by the Vite build in the container build stage.
// `all:` is required: without it go:embed silently drops files whose names start with `_` or `.`, and file-based
// routes produce chunks such as assets/_app-<hash>.js, which would then 404 in the released image.
//
//go:embed all:dist
var Dist embed.FS

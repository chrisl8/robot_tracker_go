package ui

import "embed"

// StaticFiles is the built Vue UI, embedded from internal/ui/static (the
// directory Vite builds into).
//
//go:embed static/* static/*/*
var StaticFiles embed.FS

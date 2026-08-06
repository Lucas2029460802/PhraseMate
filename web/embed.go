package web

import "embed"

// FS holds the embedded frontend assets.
//
//go:embed index.html float.html static/**
var FS embed.FS

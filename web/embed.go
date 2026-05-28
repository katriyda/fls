package web

import "embed"

//go:embed templates/*.html
//go:embed static/*
//go:embed static/fonts/*.woff2
var FS embed.FS

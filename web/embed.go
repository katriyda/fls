package web

import "embed"

//go:embed templates/*.html
//go:embed static/*
var FS embed.FS

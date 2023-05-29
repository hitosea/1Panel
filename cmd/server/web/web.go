package web

import "embed"

//go:embed manage/panel/index.html
var IndexHtml embed.FS

//go:embed manage/panel/assets/*
var Assets embed.FS

//go:embed manage/panel/index.html
var IndexByte []byte

//go:embed manage/panel/favicon.png
var Favicon embed.FS

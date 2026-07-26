package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

// templateStore 封装内嵌的 HTML 模板。
type templateStore struct {
	t *template.Template
}

func newTemplateStore() (*templateStore, error) {
	t, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &templateStore{t: t}, nil
}

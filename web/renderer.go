package web

import (
	"fmt"
	"html/template"
	"io"

	"github.com/labstack/echo"
)

type renderer struct {
	templates *template.Template
}

func (r *renderer) Render(w io.Writer, name string,
	data interface{}, c echo.Context) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func initRenderer(pages map[string]string) (*renderer, error) {
	var (
		err       error
		templates *template.Template
	)

	for name, html := range pages {
		if templates == nil {
			templates, err = template.New(name).Parse(html)
		} else {
			templates, err = templates.New(name).Parse(html)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse `%s` template: %v",
				name, err)
		}
	}

	return &renderer{templates: templates}, nil
}

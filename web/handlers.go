package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo"

	"github.com/dimuls/news-aggregator/entity"
)

func (s *Server) getIndex(c echo.Context) error {
	return c.Redirect(http.StatusPermanentRedirect, "/articles")
}

// language=HTML
const articlesPage = `<!DOCTYPE html>
<html>
<head>
	<title>Новостной агрегатор</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		input {
			width: 100%;
			box-sizing: border-box;
			padding: 0.5em;
			font-size: 1.5em;
		}
		h1 {
			padding-top: 1em 
		}
		h1:first-of-type {
			padding-top: 0
		}
		a {
			text-decoration: none;
			color: black;
		}
		p, .datetime {
			font-size: 1.5em;
		}
	</style>
</head>
<body>
	<form action="/articles" method="get">
		<input type="text" placeholder="Введите ключевые слова" name="q" value="{{.Query}}"/>
	</form>
	{{range .Articles}}
		<h1>
			<a href="{{.URL}}">
				<i>{{.SourceName}}:</i>
				{{.Header}}
			</a>
		</h1>
		<i class="datetime">{{.PublishedAt}}</i>
		{{range .Paragraphs}}
			<p>{{.}}</p>
		{{end}}
	{{else}}
		<p><i>Статей не найдено</i></p>
	{{end}}
</body>
</html>
`

type article struct {
	entity.Article
	Paragraphs  []string
	PublishedAt string
}

type articlesPageData struct {
	Query    string
	Articles []article
}

func (s *Server) getArticles(c echo.Context) error {
	query := c.QueryParam("q")

	articles, err := s.store.FindArticles(query)
	if err != nil {
		return errors.New("failed to find articles: " + err.Error())
	}

	data := articlesPageData{Query: query}

	for _, a := range articles {
		data.Articles = append(data.Articles, article{
			Article:    a,
			Paragraphs: strings.Split(a.Text, "\n"),
			// Mon Jan 2 15:04:05 -0700 MST 2006
			PublishedAt: a.PublishedAt.In(time.Local).
				Format("2006-01-02 15:04"),
		})
	}

	return c.Render(http.StatusOK, "articles", data)
}

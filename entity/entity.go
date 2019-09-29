package entity

import "time"

type Article struct {
	URL         string    `json:"url" bson:"url"`
	Header      string    `json:"header" bson:"header"`
	PublishedAt time.Time `json:"publishedAt" bson:"publishedAt"`
	Text        string    `json:"text" bson:"text"`
	SourceName  string    `json:"sourceName" bson:"sourceName"`
}

type ArticlesByPublishedAt []Article

func (as ArticlesByPublishedAt) Len() int {
	return len(as)
}

func (as ArticlesByPublishedAt) Less(i, j int) bool {
	return as[i].PublishedAt.Before(as[j].PublishedAt)
}

func (as ArticlesByPublishedAt) Swap(i, j int) {
	as[i], as[j] = as[j], as[i]
}

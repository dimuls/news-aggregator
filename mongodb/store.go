package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/globalsign/mgo/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/dimuls/news-aggregator/entity"
)

type KeywordsExtractor interface {
	ExtractKeywords(text string) ([]string, error)
}

type Store struct {
	client            *mongo.Client
	articles          *mongo.Collection
	keywordsExtractor KeywordsExtractor
}

func NewStore(mongoURI string, ke KeywordsExtractor) (*Store, error) {
	mc, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, errors.New("failed to create mongo client: " + err.Error())
	}

	err = mc.Connect(context.TODO())
	if err != nil {
		return nil, errors.New("failed to connect to mongo: " + err.Error())
	}

	err = mc.Ping(context.TODO(), readpref.Primary())
	if err != nil {
		return nil, errors.New("failed to ping mongo: " + err.Error())
	}

	db := mc.Database("newsAggregator")

	return &Store{
		client:            mc,
		articles:          db.Collection("articles"),
		keywordsExtractor: ke,
	}, nil
}

type articleWithKeywords struct {
	entity.Article `bson:",inline"`
	Keywords       []string `bson:"keywords"`
}

func (s *Store) AddArticles(as []entity.Article) error {
	var writes []mongo.WriteModel

	for _, a := range as {
		kw, err := s.keywordsExtractor.ExtractKeywords(a.Text)
		if err != nil {
			return errors.New("failed to extract keywords: " + err.Error())
		}

		writes = append(writes, mongo.NewInsertOneModel().
			SetDocument(articleWithKeywords{
				Article:  a,
				Keywords: kw,
			}))
	}

	_, err := s.articles.BulkWrite(context.TODO(), writes)
	if err != nil {
		return errors.New("failed to bulk write to mongodb: " + err.Error())
	}

	return nil
}

func (s *Store) FindArticles(query string) ([]entity.Article, error) {
	qkws, err := s.keywordsExtractor.ExtractKeywords(query)
	if err != nil {
		return nil, errors.New("failed to extract keywords from query: " +
			err.Error())
	}

	match := bson.M{}

	if len(qkws) > 0 {
		match["keywords"] = bson.M{"$all": qkws}
	}

	res, err := s.articles.Aggregate(context.TODO(), []bson.M{
		{"$match": match},
		{"$sort": bson.M{"publishedAt": -1}},
		{"$limit": 100},
	})
	if err != nil {
		return nil, errors.New("failed to aggregate: " + err.Error())
	}

	var as []entity.Article

	err = res.All(context.TODO(), &as)
	if err != nil {
		return nil, errors.New("failed to load get articles: " + err.Error())
	}

	return as, nil
}

var ErrNotFound = errors.New("not found")

func (s *Store) LatestArticle(sourceName string) (entity.Article, error) {
	res, err := s.articles.Aggregate(context.TODO(), []bson.M{
		{"$match": bson.M{
			"sourceName": sourceName,
		}},
		{"$sort": bson.M{
			"publishedAt": -1,
		}},
		{"$limit": 1},
	})
	if err != nil {
		return entity.Article{}, errors.New("failed to aggregate: " +
			err.Error())
	}

	if !res.Next(context.TODO()) {
		return entity.Article{}, ErrNotFound
	}

	var a articleWithKeywords

	err = res.Decode(&a)
	if err != nil {
		return entity.Article{}, errors.New("failed to decode article: " +
			err.Error())
	}

	return a.Article, nil
}

func (s *Store) RemoveOldArticles(to time.Time) error {
	_, err := s.articles.DeleteMany(context.TODO(), bson.M{
		"publishedAt": bson.M{"le": to},
	})
	return err
}

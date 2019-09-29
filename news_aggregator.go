package newsaggregator

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dimuls/news-aggregator/entity"
	"github.com/dimuls/news-aggregator/mongodb"
	"github.com/dimuls/news-aggregator/mystem"
	"github.com/dimuls/news-aggregator/sources/lentaru"
	"github.com/dimuls/news-aggregator/web"
)

type Source interface {
	Name() string
	Articles(from time.Time) ([]entity.Article, error)
}

type NewsAggregator struct {
	sources []Source

	store     *mongodb.Store
	webServer *web.Server

	stop       chan struct{}
	processing int32
	waitGroup  sync.WaitGroup

	log *logrus.Entry
}

func NewNewsAggregator(
	mongoURI string,
	mystemBinPath string,
	webServerBindAddr string,
) (*NewsAggregator, error) {

	ke := mystem.NewKeywordsExtractor(mystemBinPath)

	s, err := mongodb.NewStore(mongoURI, ke)
	if err != nil {
		return nil, errors.New("failed to create mongoDB store")
	}

	lentaRu, err := lentaru.NewSource()
	if err != nil {
		return nil, errors.New("failed to create lenta.ru source: " +
			err.Error())
	}

	return &NewsAggregator{
		sources: []Source{
			lentaRu,
		},
		store:     s,
		webServer: web.NewServer(webServerBindAddr, s),
		log:       logrus.WithField("subsystem", "news_aggregator"),
	}, nil
}

func (na *NewsAggregator) Start() error {
	na.stop = make(chan struct{})

	err := na.webServer.Start()
	if err != nil {
		return errors.New("failed to start web server: " + err.Error())
	}

	na.waitGroup.Add(1)
	go func() {
		defer na.waitGroup.Done()

		t := time.NewTicker(1 * time.Minute)

		for {
			na.waitGroup.Add(1)
			go func() {
				defer na.waitGroup.Done()
				na.process()
			}()

			select {
			case <-t.C:
			case <-na.stop:
				t.Stop()
				return
			}
		}
	}()

	return nil
}

func (na *NewsAggregator) Stop() {
	close(na.stop)

	na.waitGroup.Add(1)
	go func() {
		defer na.waitGroup.Done()
		na.webServer.Stop()
	}()

	na.waitGroup.Wait()
}

func (na *NewsAggregator) process() {
	if atomic.LoadInt32(&na.processing) == 1 {
		na.log.Warning("already processing")
		return
	}

	atomic.StoreInt32(&na.processing, 1)
	defer atomic.StoreInt32(&na.processing, 0)

	now := time.Now()
	na.loadNewArticles(now)
	na.removeOldArticles(now)
}

func (na *NewsAggregator) loadNewArticles(now time.Time) {
	waitGroup := sync.WaitGroup{}

	for _, s := range na.sources {
		waitGroup.Add(1)
		go func(s Source) {
			defer waitGroup.Done()

			log := na.log.WithField("source_name", s.Name())

			var from time.Time

			latestArticle, err := na.store.LatestArticle(s.Name())
			if err != nil {
				if err == mongodb.ErrNotFound {
					from = now.AddDate(0, 0, -1)
				} else {
					log.WithError(err).Error(
						"failed to get latest article for source from store")
					return
				}
			} else {
				from = latestArticle.PublishedAt.Add(1 * time.Minute)
			}

			newArticles, err := s.Articles(from)
			if err != nil {
				log.WithError(err).WithField("from", from).Error(
					"failed to get new articles from source")
				return
			}

			if len(newArticles) == 0 {
				return
			}

			err = na.store.AddArticles(newArticles)
			if err != nil {
				log.WithError(err).Error(
					"failed to add new articles to store")
				return
			}

		}(s)
	}

	waitGroup.Wait()
}

func (na *NewsAggregator) removeOldArticles(now time.Time) {
	err := na.store.RemoveOldArticles(now.AddDate(0, 0, -7))
	if err != nil {
		logrus.WithError(err).Error(
			"failed to remove old articles from store")
	}
}

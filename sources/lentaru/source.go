package lentaru

import (
	"errors"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/news-aggregator/entity"
)

const SourceName = "lenta.ru"

type Source struct {
	moscow *time.Location
	http   *http.Client
	log    *logrus.Entry
}

func NewSource() (*Source, error) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return nil, errors.New("failed to load moscow location: " + err.Error())
	}

	return &Source{
		moscow: moscow,
		http: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		log: logrus.WithField("subsystem", "lentaru_source"),
	}, nil
}

func (s *Source) Name() string {
	return SourceName
}

func (s *Source) toDateWithTime(dt time.Time, hour, minute int) time.Time {
	return time.Date(dt.Year(), dt.Month(), dt.Day(),
		hour, minute, 0, 0, s.moscow)
}

func (s *Source) Articles(from time.Time) ([]entity.Article, error) {
	now := time.Now()

	if from.After(now) {
		return nil, errors.New("from is after now")
	}

	fromDay := s.toDateWithTime(from, 0, 0)
	nowDay := s.toDateWithTime(now, 0, 0)

	as, err := s.articles(from)
	if err != nil {
		return nil, errors.New("failed to get articles: " + err.Error())
	}

	for {
		if fromDay.Equal(nowDay) {
			break
		}

		fromDay = fromDay.Add(24 * time.Hour)

		currAs, err := s.articles(fromDay)
		if err != nil {
			return nil, errors.New("failed to get articles: " + err.Error())
		}

		as = append(as, currAs...)
	}

	return as, nil
}

const baseURL = "https://lenta.ru"

func formArticlesURL(t time.Time) string {
	const articlesURL = baseURL + "/{year}/{month}/{day}/"
	asURL := strings.Replace(articlesURL, "{year}",
		strconv.Itoa(t.Year()), 1)
	asURL = strings.Replace(asURL, "{month}",
		fmt.Sprintf("%02d", t.Month()), 1)
	return strings.Replace(asURL, "{day}",
		fmt.Sprintf("%02d", t.Day()), 1)
}

func (s *Source) setDateTime(dt time.Time, timeStr string) (time.Time, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return time.Time{}, errors.New("expected 2 time parts")
	}

	hour, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, errors.New("failed to parse hour: " + err.Error())
	}

	if hour > 23 {
		return time.Time{}, errors.New(
			"failed to parse hour: should be less than 24")
	}

	minute, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, errors.New("failed to parse minute: " + err.Error())
	}

	if minute > 59 {
		return time.Time{}, errors.New(
			"failed to parse minute: should be less than 59")
	}

	return s.toDateWithTime(dt, int(hour), int(minute)), nil
}

func (s *Source) articles(from time.Time) ([]entity.Article, error) {
	asURL := formArticlesURL(from)

	log := s.log.WithField("articles_url", asURL)

	res, err := s.http.Get(asURL)
	if err != nil {
		log.WithError(err).Error("failed to get articles URL")
		return nil, errors.New("failed to HTTP get articles URL: " + err.Error())
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusFound {
			return nil, nil
		}
		log.WithField("status_code", res.StatusCode).
			Error("get articles returned not OK status code")
		return nil, errors.New("not OK status code")
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, errors.New("failed to parse articles HTML: " + err.Error())
	}

	var (
		as      []entity.Article
		findErr error
	)

	doc.Find(".item.news").EachWithBreak(
		func(_ int, sel *goquery.Selection) bool {
			urlPath, urlPathExists := sel.Find(".titles > h3 > a").
				Attr("href")
			if !urlPathExists {
				findErr = errors.New("failed to find article URL")
				return false
			}

			timeStr := strings.TrimSpace(sel.Find(".time").Text())

			publishedAt, err := s.setDateTime(from, timeStr)
			if err != nil {
				findErr = errors.New("failed to set date time: " + err.Error())
				return false
			}

			headerEnc := strings.TrimSpace(
				sel.Find(".titles > h3 > a > span").Text())

			as = append(as, entity.Article{
				URL:         baseURL + urlPath,
				Header:      html.UnescapeString(headerEnc),
				PublishedAt: publishedAt,
				SourceName:  SourceName,
			})

			return true
		})
	if findErr != nil {
		return nil, errors.New("failed to find all articles: " +
			findErr.Error())
	}

	sort.Sort(entity.ArticlesByPublishedAt(as))

	fromIndex := -1

	for i, a := range as {
		if a.PublishedAt.Equal(from) || a.PublishedAt.After(from) {
			fromIndex = i
			break
		}
	}

	if fromIndex == -1 {
		return nil, nil
	}

	as = as[fromIndex:]

	for i := range as {
		as[i].Text, err = s.articleText(as[i].URL)
		if err != nil {
			return nil, errors.New("failed to get article text: " + err.Error())
		}
	}

	return as, nil
}

func (s *Source) articleText(aURL string) (string, error) {
	res, err := s.http.Get(aURL)
	if err != nil {
		return "", errors.New("failed to HTTP get article URL: " + err.Error())
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.New("not OK status code")
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", errors.New("failed to parse article HTML: " + err.Error())
	}

	var ps []string

	doc.Find(".b-text > p").Each(func(i int, s *goquery.Selection) {
		ps = append(ps,
			strings.TrimSpace(html.UnescapeString(s.Text())))
	})

	return strings.Join(ps, "\n"), nil
}

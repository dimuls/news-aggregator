package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sirupsen/logrus"

	"github.com/dimuls/news-aggregator/entity"
)

type Store interface {
	FindArticles(query string) ([]entity.Article, error)
}

type Server struct {
	bindAddr string
	store    Store

	echo *echo.Echo

	waitGroup sync.WaitGroup

	log *logrus.Entry
}

func NewServer(bindAddr string, s Store) *Server {

	return &Server{
		bindAddr: bindAddr,
		store:    s,

		log: logrus.WithField("subsystem", "web_server"),
	}
}

func (s *Server) Start() error {
	e := echo.New()

	e.HideBanner = true
	e.HidePort = true

	var err error

	e.Renderer, err = initRenderer(map[string]string{
		"articles": articlesPage,
	})
	if err != nil {
		return errors.New("failed to init renderer: " + err.Error())
	}

	e.Use(middleware.Recover())
	e.Use(logrusLogger)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		var (
			code = http.StatusInternalServerError
			msg  interface{}
		)

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			msg = he.Message
		} else if e.Debug {
			msg = err.Error()
		} else {
			msg = http.StatusText(code)
		}
		if _, ok := msg.(string); !ok {
			msg = fmt.Sprintf("%v", msg)
		}

		// Send response
		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead { // Issue #608
				err = c.NoContent(code)
			} else {
				err = c.String(code, msg.(string))
			}
			if err != nil {
				s.log.WithError(err).Error("failed to error response")
			}
		}
	}

	e.GET("/", s.getIndex)
	e.GET("/articles", s.getArticles)

	s.echo = e

	s.waitGroup.Add(1)
	go func() {
		defer s.waitGroup.Done()
		err := e.Start(s.bindAddr)
		if err != nil && err != http.ErrServerClosed {
			s.log.WithError(err).Error("failed to start")
		}
	}()

	return nil
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	err := s.echo.Shutdown(ctx)
	if err != nil {
		s.log.WithError(err).Error("failed to graceful stop")
	}

	s.waitGroup.Wait()
}

func logrusLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		err := next(c)

		stop := time.Now()

		if err != nil {
			c.Error(err)
		}

		req := c.Request()
		res := c.Response()

		p := req.URL.Path
		if p == "" {
			p = "/"
		}

		bytesIn := req.Header.Get(echo.HeaderContentLength)
		if bytesIn == "" {
			bytesIn = "0"
		}

		entry := logrus.WithFields(map[string]interface{}{
			"subsystem":    "web_server",
			"remote_ip":    c.RealIP(),
			"host":         req.Host,
			"query_params": c.QueryParams(),
			"uri":          req.RequestURI,
			"method":       req.Method,
			"path":         p,
			"referer":      req.Referer(),
			"user_agent":   req.UserAgent(),
			"status":       res.Status,
			"latency":      stop.Sub(start).String(),
			"bytes_in":     bytesIn,
			"bytes_out":    strconv.FormatInt(res.Size, 10),
		})

		const msg = "request handled"

		if res.Status >= 500 {
			if err != nil {
				entry = entry.WithError(err)
			}
			entry.Error(msg)
		} else if res.Status >= 400 {
			entry.Warn(msg)
		} else {
			entry.Info(msg)
		}

		return nil
	}
}

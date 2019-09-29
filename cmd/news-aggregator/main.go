package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	newsaggregator "github.com/dimuls/news-aggregator"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	newsAggr, err := newsaggregator.NewNewsAggregator(
		os.Getenv("NEWS_AGGREGATOR_MONGODB_URI"),
		os.Getenv("NEWS_AGGREGATOR_MYSTEM_BIN_PATH"),
		os.Getenv("NEWS_AGGREGATOR_WEB_SERVER_BIND_ADDR"))
	if err != nil {
		logrus.WithError(err).Fatal("failed to create news aggregator")
	}

	err = newsAggr.Start()
	if err != nil {
		logrus.WithError(err).Fatal("failed to start news aggregator")
	}

	time.Sleep(200 * time.Millisecond)

	ss := make(chan os.Signal)
	signal.Notify(ss, os.Interrupt, syscall.SIGTERM)

	s := <-ss

	logrus.Infof("captured %v signal, stopping", s)

	st := time.Now()

	newsAggr.Stop()

	et := time.Now()

	logrus.Infof("stopped in %g seconds, exiting",
		et.Sub(st).Seconds())
}

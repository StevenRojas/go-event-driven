package main

import (
	"StevenRojas/go-reporting/generators/excel/pkg/service"
	"os"
	"os/signal"
	"syscall"

	glog "github.com/StevenRojas/go-logger-wrapper"
	goredis "github.com/StevenRojas/go-redis-mq"
)

const generatorChannel = "generator_channel_excel"

func main() {
	logger, err := glog.NewLogger()
	if err != nil {
		panic(err)
	}

	redis, err := goredis.InitClient()
	if err != nil {
		logger.Error("Error connecting Redis", err)
		panic(err)
	}

	s := service.NewService(redis, logger)
	s.ListenForJobs(generatorChannel)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	defer signal.Stop(signals)
	<-signals
	go func() {
		<-signals
		os.Exit(1)
	}()

}

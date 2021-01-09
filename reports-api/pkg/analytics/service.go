package analytics

import (
	glog "github.com/StevenRojas/go-logger-wrapper"
	goredis "github.com/StevenRojas/go-redis-mq"
)

type AnalyticsService interface {
	SalesReportExcel(dispatchTo string) string
	SalesReportPDF(dispatchTo string, orderID int) string
	RevenueReport(dispatchTo string) string
}

type analyticsService struct {
	repo   AnalyticsRepository
	redis  goredis.RedisWrapper
	logger glog.LoggerWrapper
}

func NewAnalyticsService(
	analyticsRepository AnalyticsRepository,
	redisClient goredis.RedisWrapper,
	logger glog.LoggerWrapper,
) AnalyticsService {
	return &analyticsService{
		repo:   analyticsRepository,
		redis:  redisClient,
		logger: logger,
	}
}

func (as *analyticsService) SalesReportExcel(dispatchTo string) string {
	report := NewSalesService(as.repo)
	queueName, job := report.SalesReportExcel(dispatchTo)

	as.logger.Info("About to send a job into queue " + queueName)
	errorChan := make(chan error) // TODO: listen for errors in the messaging queue
	queue, err := as.redis.CreateQueue(queueName, errorChan)
	if err != nil {
		as.logger.Error("Error creating queue", queueName, err)
	}
	queue.Publish(job)

	code := "sre123" // TODO: Logic to generate an identifier for the report
	return code
}

func (as *analyticsService) SalesReportPDF(dispatchTo string, orderID int) string {
	// report := NewSalesService(as.repo)
	// queue, job := report.SalesReportPDF(dispatchTo, orderID)
	code := "srp456" // TODO: Logic to generate an identifier for the report
	return code
}

func (as *analyticsService) RevenueReport(dispatchTo string) string {
	return "To be implemented"
}

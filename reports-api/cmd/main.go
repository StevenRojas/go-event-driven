package main

import (
	"StevenRojas/go-reporting/reports-api/pkg/analytics"
	"encoding/json"
	"net/http"
	"strconv"

	glog "github.com/StevenRojas/go-logger-wrapper"
	goredis "github.com/StevenRojas/go-redis-mq"
)

type response struct {
	Message string `json:"message"`
	Key     string `json:"key"`
}

func main() {
	mux := http.NewServeMux()

	logger, err := glog.NewLogger()
	if err != nil {
		panic(err)
	}
	redisClient, err := goredis.InitClient()
	if err != nil {
		logger.Error("Error connecting Redis", err)
		panic(err)
	}

	analyticsRepository := analytics.NewAnalyticsRepository()
	analyticsService := analytics.NewAnalyticsService(analyticsRepository, redisClient, logger)

	mux.HandleFunc("/analytics/mysql/excel/email", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)

		code := analyticsService.SalesReportExcel("email")

		json.NewEncoder(res).Encode(response{
			Message: "Generating a report using MYSQL as collector and redering an Excel that is send by Email",
			Key:     code,
		})
	})

	mux.HandleFunc("/analytics/mysql/pdf/s3", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)

		params := req.URL.Query()
		p, _ := params["orderId"]
		orderID, _ := strconv.Atoi(p[0])
		code := analyticsService.SalesReportPDF("S3", orderID)

		json.NewEncoder(res).Encode(response{
			Message: "Generating a report using a MYSQL as collector and redering a PDF that uploaded to S3",
			Key:     code,
		})
	})

	mux.HandleFunc("/suites/api/excel/s3", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)

		// code := analyticsService.ApiExcelEmailReport()

		json.NewEncoder(res).Encode(response{
			Message: "Generating a report using an API service as collector and redering an Excel that uploaded to S3",
			Key:     "",
		})
	})

	mux.HandleFunc("/suites/api/excel/s3/cache", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)

		// code := analyticsService.ApiExcelEmailReport()

		json.NewEncoder(res).Encode(response{
			Message: "Generating a report using an API service with cache as collector and redering an Excel that uploaded to S3",
			Key:     "",
		})
	})

	logger.Info("Listen for requests at por :9000...")
	http.ListenAndServe(":9000", mux)
}

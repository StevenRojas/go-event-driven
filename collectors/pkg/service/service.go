package service

import (
	"StevenRojas/go-reporting/collectors/pkg/collector"
	"StevenRojas/go-reporting/collectors/pkg/protos/collectors"
	"StevenRojas/go-reporting/collectors/pkg/protos/jobs"

	glog "github.com/StevenRojas/go-logger-wrapper"
	goredis "github.com/StevenRojas/go-redis-mq"
	proto "github.com/golang/protobuf/proto"
)

type Service interface {
	ListenForJobs(queueName string) error
}

type service struct {
	redis  goredis.RedisWrapper
	logger glog.LoggerWrapper
}

func NewService(redis goredis.RedisWrapper, logger glog.LoggerWrapper) Service {
	return &service{
		redis:  redis,
		logger: logger,
	}
}

func (s *service) ListenForJobs(queueName string) error {
	errorChan := make(chan error) // TODO: listen for errors in the messaging queue
	// Create a messaging queue for listen jobs
	queue, err := s.redis.CreateQueue(queueName, errorChan)
	if err != nil {
		s.logger.Error("Error creating queue", err)
		return err
	}
	receiver := make(chan []byte)
	// Go routine that process the incoming messages
	go s.processMessages(receiver)
	// Add a consumer to the messaging queue
	queue.Subscribe(receiver, errorChan)
	s.logger.Info("Listen for API Collector jobs...")
	return nil
}

func (s *service) processMessages(receiver chan []byte) {
	for message := range receiver {
		s.logger.Info("Message get from queue")
		var job jobs.Job
		if err := proto.Unmarshal(message, &job); err != nil {
			s.logger.Error("Error unmarshalling the message into a Job", err)
		}
		s.logger.Info("Job name: " + job.GetName())
		s.logger.Info("Job pattern: " + job.Pattern)

		go s.collect(&job)
		// Send next job in stack including the streamName
		go s.sendNextJob(job.Next)

	}
}

func (s *service) collect(job *jobs.Job) {
	feedStream := s.redis.CreateStream(job.StreamName, 1)
	switch job.Pattern {
	case "mysql":
		var task collectors.MysqlCollectorTask
		if err := proto.Unmarshal(job.Task.Value, &task); err != nil {
			s.logger.Info("Unable to deserialize task into MysqlCollector")
		}
		mysqlCollector, _ := collector.NewMySQLCollector(&task, feedStream, s.logger)
		s.logger.Info("Collect data and send it to stream", job.StreamName)
		go mysqlCollector.Collect()
	case "api":
		// TODO: add collector
	default:
		// TODO: notify about unsupported collector type
	}
}

func (s *service) sendNextJob(job *jobs.Job) {
	if job == nil {
		return
	}
	s.logger.Info("Sending next job", job.Channel)
	errorChan := make(chan error) // TODO: listen for errors in the messaging queue
	queue, err := s.redis.CreateQueue(job.Channel, errorChan)
	if err != nil {
		s.logger.Error("Error creating queue", job.Channel, err)
	}
	serJob, _ := proto.Marshal(job)
	queue.Publish(serJob)
}

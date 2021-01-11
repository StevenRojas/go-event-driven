package service

import (
	"StevenRojas/go-reporting/generators/excel/pkg/protos/jobs"
	"fmt"

	glog "github.com/StevenRojas/go-logger-wrapper"
	goredis "github.com/StevenRojas/go-redis-mq"
	"google.golang.org/protobuf/proto"
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
	errorChan := make(chan error)
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
	s.logger.Info("Listen for Generator jobs...")
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

		go s.listenForData(&job)

		// TODO: Send next job in stack of jobs when the file is ready
		//go s.sendNextJob(job.Next)

	}
}

func (s *service) listenForData(job *jobs.Job) {
	feedStream := s.redis.CreateStream(job.StreamName, 0)
	go feedStream.Consume(0)

	go func() {
		for {
			select {
			case m := <-feedStream.MessageChannel():
				fmt.Printf("message from stream: %v\n", m)
			case e := <-feedStream.ErrorChannel():
				fmt.Printf("error from stream: %v\n", e)
			case f := <-feedStream.FinishedChannel():
				fmt.Printf("Finish collecting data %v\n", f)
			}
		}
	}()
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

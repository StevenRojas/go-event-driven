

# go-reports

Go Reports is a set of collectors, generators, dispatchers and other auxiliar microservices that allows generate reports in different formats, collecting data from different sources and dispatching the reports to different targets, with an Event-Driven architecture using a Redis Messaging Queue.

![Arhitecture](https://github.com/StevenRojas/go-reporting/blob/main/go-reports.png)

## Installation
```go
go get github.com/StevenRojas/go-reports
```

## Setup environment
### JWT
```go
export JWT_SECRET_KEY=secret!
export JWT_EXPIRE_HOURS=2
export JWT_REFRESH_HOURS=7
```
### Redis
```go
export REDIS_ADDR=localhost:6379
export REDIS_DB=10
export REDIS_PASS=secret
```

## Reports API
A Go-Kit implementation that handles REST and gRPC requests to generate reports. Each endpoint calls a service which creates a `Job` that will be sent to the `Messaging Queue`. The `Job` has all the information needed to collect the data, generate the report and send it to a target. 

### Jobs
A `Job` consist on a `task` that should be performed by a microserive, a `channel` to be sent in the `Messaging Queue`, a `pattern` that is used to select an specific microservice that should perform the task and other `Jobs` that should be executed after the current taks is completed. The `Job` is defined using `proto buffers`
```go
serTask, _ := proto.Marshal(task)
job := &jobs.Job{
	Name: "SalesReportExcel-collector",
	Channel: collectorChannel,
	Pattern: "mysql",
	Task: &any.Any{
		TypeUrl: typeUrlPrefix + proto.MessageName(task),
		Value: serTask,
	},
	Next: getOtherJob(),
}
```
### Tasks
Task is any struct that contains the information needed by a microservice to be executed. It could be for example a `task` that will collect data from MySQL
```go
var queryList []*collectors.Query
for name, sql := range s.repo.MySQLExcelEmailQuery() {
	query := &collectors.Query{
		Name: name,
		Sql: sql,
		UsePagination: false,
	}
	queryList = append(queryList, query)
}
task := &collectors.MysqlCollector{
	Instance: "SalesReportExcel-mysql-collector",
	QueryList: queryList,
	DbName: "connect",
	UseCache: true,
	CredentialsId: "abc-123",
}
```
### Concatenated Jobs
As mentioned earlier, a `Job` could have a sequence of `jobs` that will be executed once the previous is done. With this approach, is possible define a sequence like:
`Collect > Generate > Dispatch` 
or
 `Collect > Validate > Integrate > Generate > Dispatch > Notify`
 So, when a `Job` is performed, the last operation of the microservice which is executing the related `Task` is to send `Next` to the `Messaging Queue`

### Service example
```go
func (as *analyticsService) SalesReportExcel(dispatchTo string) string {
  report := NewSalesService(as.repo)
  queueName, job := report.SalesReportExcel(dispatchTo)

  as.logger.Info("About to send a job into queue " + queueName)
  errorChan := make(chan error)
  queue, err := as.redis.CreateQueue(queueName, errorChan)
  if err != nil {
    as.logger.Error("Error sending job to queue", queueName, err)
  }
  queue.PublishBytes(job)
}

func (s *salesService) SalesReportExcel(dispatchTo string) (string, []byte) {
  collectorJob := s.createMysqlCollectorJob()
  generatorJob := s.createExcelGeneratorJob()
  switch dispatchTo {
  case "S3":
    generatorJob.Next = s.createS3DispatcherJob()
  case "email":
    generatorJob.Next = s.createEmailDispatcherJob()
  }
  collectorJob.Next = generatorJob
  serJob, _ := proto.Marshal(collectorJob)
  return collectorJob.GetChannel(), serJob
}
```

## Collector microservices
The goal of this set of services is to collect the data needed for each report. There are generic microservices that could connect to specific databases or perform calls to APIs. On the other hand, it is possible to add specialized microservices that collects the information with extended business logic.

`Collectors` sends the collected data into `Messaging Queue` using `Redis Streams` so that if the `Generator` service is down, it can get the data when is it back. The `Collector` service will create a new stream, publish the stream name (so that the `Generator` will know which stream should listen) and then send the dataset row by row.

### Listen for jobs in Messaging Queue
```go
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
	s.logger.Info("Listen for Collector jobs...")
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
		// Send next job in stack of jobs
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
```

### Send dataset into stream
```go
func (c *mysqlCollector) Collect() {
	log := fmt.Sprintf("Collecting from database [%s] using cache [%t] and credential IDs [%s]",
		c.task.DbName, c.task.UseCache, c.task.CredentialsId)
	c.logger.Info(log)
	c.logger.Info("running the following queries")
	for _, query := range c.task.QueryList {
		log = fmt.Sprintf("Query name [%s]; using pagination [%t]; query: [%s]",
			query.Name, query.UsePagination, query.Sql)
		c.logger.Info(log)
		c.runQuery(query.Sql)
	}
}

func (c *mysqlCollector) runQuery(sql string) {
	c.logger.Info("Query executed, returning results")
	var lastID string
	// Fake data
	for i := 1; i <= 15; i++ {
		result := make(map[string]interface{})
		result["id"] = i
		result["name"] = fmt.Sprintf("Name-%d", i)
		result["email"] = fmt.Sprintf("email%d@g.com", i)
		result["active"] = i%2 == 0

		// Binary marshal
		b, err := msgpack.Marshal(result)
		// Send to stream
		lastID, err = c.stream.Publish(b)
		if err != nil {
			c.logger.Error("Unable to marshal stream message", err)
		}
		fmt.Printf("Published row : %v\n", result)
		time.Sleep(500 * time.Millisecond)
	}
	c.logger.Info("Last stream ID: " + lastID)
	// TODO: send message with the last stream ID
}
```
### Sending next jobs to the messaging queue
```go
func (s *service) sendNextJob(job *jobs.Job) {
	if job == nil {
		return
	}
	s.logger.Info("Sending next job", job.Channel)
	errorChan := make(chan error)
	queue, err := s.redis.CreateQueue(job.Channel, errorChan)
	if err != nil {
		s.logger.Error("Error creating queue", job.Channel, err)
	}
	serJob, _ := proto.Marshal(job)
	queue.Publish(serJob)
}
```

## Generator microservices
The goal of these microservices is use the data that is comming from  `Collectors` through the `Messaging Queue` and generate a report in an specific format. There are genercic microservices that uses predefined templates with `Mustache` and also specialized generators. The files are stored in a temporal location where the `Dispatchers` will get them 

```go
func (s *service) ListenForJobs(queueName string) error {
  ...
  go s.processMessages(receiver)
  ...
}

func (s *service) processMessages(receiver chan []byte) {
   ...
   go s.listenForData(&job)
   ...
}

func (s *service) listenForData(job *jobs.Job) {
	feedStream := s.redis.CreateStream(job.StreamName, 0)
	go feedStream.Consume(0)

	go func() {
		for {
			select {
			case m := <-feedStream.MessageChannel():
				fmt.Printf("message from stream: %v\n", m)
				// TODO: process message (i.e.: add row to excel)
			case e := <-feedStream.ErrorChannel():
				fmt.Printf("error from stream: %v\n", e)
				// TODO: process error
			case f := <-feedStream.FinishedChannel():
				fmt.Printf("Finish collecting data %v\n", f)
				// TODO: store the file in temp location and send next job
			}
		}
	}()
}

```

## Dispatcher microservices
Once the `Generator` sevice creates a report file, one of this set of services dispatch the file to the target, which could be an Email, AWS S3, Drive, etc., or even an API.

## Other microservices
It is possible to add any microservice that listen for Jobs in the `Messaging Queue`and works over the data, keeping in mind that a `Job's task` is defined as an abstract structure, there is a lot of flexibility for process the data that is flowing. For example:

 - **Validators:** which can perform a `Broken Access Control`of the collected data against a `JWT`
 - **Agregators:** which can combine the result of multiple queries before sending them to the `Collectors`
 - **Notifiers:** which can notify by Email or an API call that a report is ready or that something went wrong
 - **Monitoring:** which can collect metrics, store audit logs, send alerts, etc.
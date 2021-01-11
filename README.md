

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
func (as *apiService) ListenForJobs(queueName string) error {
  errorChan := make(chan error)
  queue, err := as.redis.CreateQueue(queueName, errorChan)
  if err != nil {
    as.logger.Error("Error creating queue", err)
    return err
  }
  receiver := make(chan []byte)
  go as.processMessages(receiver)
  queue.CreateConsumer(receiver, errorChan)
  as.logger.Info("Listen for API Collector jobs...")
  return nil
}

func (as *apiService) processMessages(receiver chan []byte) {
  for message := range receiver {
    as.logger.Info("Message get from queue")
    var job jobs.Job
    if err := proto.Unmarshal(message, &job); err != nil {
      as.logger.Error("Error unmarshalling the message into a Job", err)
    }
    as.logger.Info("Job name: " + job.GetName())
    as.logger.Info("Job pattern: " + job.GetPattern())
  }
}
```

### Send dataset into stream
```go
```

## Generator microservices
The goal of these microservices is use the data that is comming from  `Collectors` through the `Messaging Queue` and generate a report in an specific format. There are genercic microservices that uses predefined templates with `Mustache` and also specialized generators. The files are stored in a temporal location where the `Dispatchers` will get them 

## Dispatcher microservices
Once the `Generator` sevice creates a report file, one of this set of services dispatch the file to the target, which could be an Email, AWS S3, Drive, etc., or even an API.

## Other microservices
It is possible to add any microservice that listen for Jobs in the `Messaging Queue`and works over the data, keeping in mind that a `Job's task` is defined as an abstract structure, there is a lot of flexibility for process the data that is flowing. For example:

 - **Validators:** which can perform a `Broken Access Control`of the collected data against a `JWT`
 - **Agregators:** which can combine the result of multiple queries before sending them to the `Collectors`
 - **Notifiers:** which can notify by Email or an API call that a report is ready or that something went wrong
 - **Monitoring:** which can collect metrics, store audit logs, send alerts, etc.
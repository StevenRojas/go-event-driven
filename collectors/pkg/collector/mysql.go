package collector

import (
	"StevenRojas/go-reporting/collectors/pkg/protos/collectors"
	"fmt"
	"time"

	glog "github.com/StevenRojas/go-logger-wrapper"
	goredis "github.com/StevenRojas/go-redis-mq"
	"github.com/vmihailenco/msgpack/v5"
)

type MySQLCollector interface {
	Collect()
}

type mysqlCollector struct {
	task   *collectors.MysqlCollectorTask
	stream goredis.RedisStreamWrapper
	logger glog.LoggerWrapper
}

func NewMySQLCollector(task *collectors.MysqlCollectorTask, stream goredis.RedisStreamWrapper, logger glog.LoggerWrapper) (MySQLCollector, error) {
	return &mysqlCollector{
		task:   task,
		stream: stream,
		logger: logger,
	}, nil
}

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
	for i := 1; i <= 5; i++ {
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

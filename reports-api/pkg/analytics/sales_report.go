package analytics

import (
	"StevenRojas/go-reporting/reports-api/pkg/protos/collectors"
	"StevenRojas/go-reporting/reports-api/pkg/protos/dispatchers"
	"StevenRojas/go-reporting/reports-api/pkg/protos/generators"
	"StevenRojas/go-reporting/reports-api/pkg/protos/jobs"
	"StevenRojas/go-reporting/reports-api/pkg/protos/mappers"
	"fmt"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
)

const collectorChannel = "collector_channel"
const generatorChannelPrefix = "generator_channel_"
const dispatcherChannelPrefix = "dispatcher_channel_"
const typeUrlPrefix = "StevenRojas/go-reporting/"
const reportTmpFilePath = "/tmp/reports/analytics/"
const templateFilePath = "/templates/analytics/"
const targetFilename = "test.xlsx" // TODO: Define logic to generate filenames

type SalesService interface {
	SalesReportExcel(dispatchTo string) (string, []byte)
	SalesReportPDF(dispatchTo string, orderID int) (string, []byte)
}

type salesService struct {
	repo AnalyticsRepository
}

func NewSalesService(analyticsRepository AnalyticsRepository) SalesService {
	return &salesService{
		repo: analyticsRepository,
	}
}

func (s *salesService) SalesReportExcel(dispatchTo string) (string, []byte) {
	streamName := fmt.Sprintf("SalesReportExcel-%d", time.Now().Unix())
	collectorJob := s.createMysqlCollectorJob(streamName)
	generatorJob := s.createExcelGeneratorJob(streamName)
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

func (s *salesService) SalesReportPDF(dispatchTo string, orderID int) (string, []byte) {
	return "", nil
}

func (s *salesService) createMysqlCollectorJob(streamName string) *jobs.Job {
	var queryList []*collectors.Query
	for name, sql := range s.repo.MySQLExcelEmailQuery() {
		query := &collectors.Query{
			Name:          name,
			Sql:           sql,
			UsePagination: false,
		}
		queryList = append(queryList, query)
	}

	task := &collectors.MysqlCollectorTask{
		Instance:      "SalesReportExcel-mysql-collector",
		QueryList:     queryList,
		DbName:        "connect",
		UseCache:      true,
		CredentialsId: "abc-123",
	}
	serTask, _ := proto.Marshal(task)

	job := &jobs.Job{
		Name:       "SalesReportExcel-collector",
		Channel:    collectorChannel,
		Pattern:    "mysql",
		StreamName: streamName,
		Task: &any.Any{
			TypeUrl: typeUrlPrefix + proto.MessageName(task),
			Value:   serTask,
		},
	}
	return job
}

func (s *salesService) createExcelGeneratorJob(streamName string) *jobs.Job {
	columns := make(map[string]mappers.ExcelColumn)
	columns["id"] = mappers.ExcelColumn{
		ColumnName: "A",
		ColumnType: mappers.DataTypeEnum_INTEGER,
	}
	defineColunmsMapper()
	templateFilename := "salesReport.xlsx"
	task := &generators.Generator{
		Instance:       "SalesReportExcel-excel-generator",
		Format:         generators.FormatEnum_XLSX,
		ReportFilepath: reportTmpFilePath + targetFilename,
		Template: &generators.Template{
			Name:     "Analytics - Sales Report",
			Folder:   templateFilePath,
			Filename: templateFilename,
		},
	}
	serTask, _ := proto.Marshal(task)

	job := &jobs.Job{
		Name:       "SalesReportExcel-generator",
		Channel:    generatorChannelPrefix + "excel",
		StreamName: streamName,
		Task: &any.Any{
			TypeUrl: typeUrlPrefix + proto.MessageName(task),
			Value:   serTask,
		},
	}
	return job
}

func (s *salesService) createS3DispatcherJob() *jobs.Job {
	var reportList []*dispatchers.ReportFile
	reportList = append(reportList, &dispatchers.ReportFile{
		ReportFilepath: reportTmpFilePath + targetFilename,
	})
	task := &dispatchers.S3DispatcherTask{
		Instance:  "SalesReport-dispatcher-S3",
		SecretKey: "123",
		AccessKey: "abc",
		Token:     "a1b2c3",
		Region:    "west",
		Bucket:    "bucket1",
		Report:    reportList,
	}
	serTask, _ := proto.Marshal(task)

	job := &jobs.Job{
		Name:    "SalesReport-dispatcher",
		Channel: dispatcherChannelPrefix + "S3",
		Task: &any.Any{
			TypeUrl: typeUrlPrefix + proto.MessageName(task),
			Value:   serTask,
		},
	}
	return job
}

func (s *salesService) createEmailDispatcherJob() *jobs.Job {
	var reportList []*dispatchers.ReportFile
	reportList = append(reportList, &dispatchers.ReportFile{
		ReportFilepath: reportTmpFilePath + targetFilename,
	})
	task := &dispatchers.EmailDispatcherTask{
		Instance:  "SalesReport-dispatcher-Email",
		Subject:   "Sales Report",
		Body:      "This is a test of Sales Report",
		IsHtml:    false,
		Recipient: []string{"steven.rojas@gmail.com"},
		Cc:        []string{"steven.rojas@avantica.net", "steven.rojas@appetize.com"},
		Report:    reportList,
	}
	serTask, _ := proto.Marshal(task)

	job := &jobs.Job{
		Name:    "SalesReport-dispatcher",
		Channel: dispatcherChannelPrefix + "email",
		Task: &any.Any{
			TypeUrl: typeUrlPrefix + proto.MessageName(task),
			Value:   serTask,
		},
	}
	return job
}

func defineColunmsMapper() *mappers.ExcelDataMapper {
	columns := make(map[string]*mappers.ExcelColumn)
	columns["id"] = &mappers.ExcelColumn{
		ColumnName: "A",
		ColumnType: mappers.DataTypeEnum_INTEGER,
	}
	columns["name"] = &mappers.ExcelColumn{
		ColumnName: "B",
		ColumnType: mappers.DataTypeEnum_STRING,
	}
	columns["email"] = &mappers.ExcelColumn{
		ColumnName: "C",
		ColumnType: mappers.DataTypeEnum_STRING,
	}
	columns["active"] = &mappers.ExcelColumn{
		ColumnName: "D",
		ColumnType: mappers.DataTypeEnum_BOOLEAN,
	}

	return &mappers.ExcelDataMapper{
		Columns: columns,
	}
}

package analytics

import (
	"fmt"
)

type AnalyticsRepository interface {
	MySQLExcelEmailQuery() map[string]string
	MySQLPdfS3Query(orderID int) map[string]string
}

type analyticsRepository struct {
}

func NewAnalyticsRepository() AnalyticsRepository {
	return &analyticsRepository{}
}

func (aq *analyticsRepository) MySQLExcelEmailQuery() map[string]string {
	query := map[string]string{}
	query["salesByItem"] = "SELECT * FROM sales	WHERE item IS NOT NULL"
	return query
}

func (aq *analyticsRepository) MySQLPdfS3Query(orderID int) map[string]string {
	query := map[string]string{}
	query["revenue"] = `
		SELECT * 
		FROM revenue
		WHERE revenueId > 0
	`

	query["orders"] = fmt.Sprintf(`
		SELECT * 
		FROM orders
		WHERE orderId = %d
	`, orderID)

	return query
}

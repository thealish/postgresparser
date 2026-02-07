// Example: basic usage of postgresparser.ParseSQL.
package main

import (
	"fmt"
	"log"

	"github.com/valkdb/postgresparser"
)

func main() {
	sql := `SELECT o.id, o.total, c.name
FROM orders o
JOIN customers c ON c.id = o.customer_id
WHERE o.status = 'active'
ORDER BY o.created_at DESC
LIMIT 10`

	result, err := postgresparser.ParseSQL(sql)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Command: %s\n", result.Command)
	fmt.Printf("Tables:  %v\n", result.Tables)
	fmt.Printf("Columns: %v\n", result.Columns)
	fmt.Printf("Joins:   %v\n", result.JoinConditions)
	fmt.Printf("Where:   %v\n", result.Where)
	fmt.Printf("OrderBy: %v\n", result.OrderBy)
}

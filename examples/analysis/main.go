// Example: using the analysis subpackage for SQL analysis.
package main

import (
	"fmt"
	"log"

	"github.com/valkdb/postgresparser/analysis"
)

func main() {
	sql := `SELECT o.id, o.total, c.name, c.email
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.status = 'active' AND o.total > 100
ORDER BY o.created_at DESC`

	// Full SQL analysis in a single pass.
	result, err := analysis.AnalyzeSQL(sql)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Tables ===")
	for _, t := range result.Tables {
		fmt.Printf("  %s (alias: %s)\n", t.Name, t.Alias)
	}

	fmt.Println("\n=== Column Usage ===")
	for _, cu := range result.ColumnUsage {
		fmt.Printf("  %s.%s → %s\n", cu.TableAlias, cu.Column, cu.UsageType)
	}

	// WHERE condition extraction.
	fmt.Println("\n=== WHERE Conditions ===")
	conditions, err := analysis.ExtractWhereConditions(sql)
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range conditions {
		fmt.Printf("  %s.%s %s %v\n", c.Table, c.Column, c.Operator, c.Value)
	}

	// JOIN relationship detection with schema metadata.
	fmt.Println("\n=== JOIN Relationships (with schema) ===")
	schema := map[string][]analysis.ColumnSchema{
		"customers": {
			{Name: "id", PGType: "bigint", IsPrimaryKey: true},
			{Name: "name", PGType: "text"},
			{Name: "email", PGType: "text"},
		},
		"orders": {
			{Name: "id", PGType: "bigint", IsPrimaryKey: true},
			{Name: "customer_id", PGType: "bigint"},
			{Name: "status", PGType: "text"},
			{Name: "total", PGType: "numeric"},
		},
	}

	joins, err := analysis.ExtractJoinRelationshipsWithSchema(sql, schema)
	if err != nil {
		log.Fatal(err)
	}
	for _, j := range joins {
		fmt.Printf("  %s.%s → %s.%s\n",
			j.ChildTable, j.ChildColumn,
			j.ParentTable, j.ParentColumn)
	}
}

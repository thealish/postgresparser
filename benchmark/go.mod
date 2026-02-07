module github.com/valkdb/postgresparser/benchmark

go 1.25.6

require (
	github.com/pganalyze/pg_query_go/v6 v6.0.0
	github.com/valkdb/postgresparser v0.0.0
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

replace github.com/valkdb/postgresparser => ..

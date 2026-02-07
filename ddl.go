// ddl.go implements DDL population logic for DROP, ALTER TABLE, CREATE INDEX, and TRUNCATE.
package postgresparser

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"

	"github.com/valkdb/postgresparser/gen"
)

// populateDropStmt handles DROP TABLE, DROP INDEX, and DROP INDEX CONCURRENTLY.
func populateDropStmt(result *ParsedQuery, ctx gen.IDropstmtContext, tokens antlr.TokenStream) error {
	if ctx == nil {
		return fmt.Errorf("drop statement: %w", ErrNilContext)
	}

	// Determine shared flags.
	var flags []string
	ifExists := ctx.IF_P() != nil && ctx.EXISTS() != nil
	if ifExists {
		flags = append(flags, "IF_EXISTS")
	}
	if db := ctx.Drop_behavior_(); db != nil {
		if db.CASCADE() != nil {
			flags = append(flags, "CASCADE")
		} else if db.RESTRICT() != nil {
			flags = append(flags, "RESTRICT")
		}
	}

	// DROP INDEX CONCURRENTLY (special grammar alternatives).
	if ctx.CONCURRENTLY() != nil {
		flags = append(flags, "CONCURRENTLY")
		if nameList := ctx.Any_name_list_(); nameList != nil {
			for _, anyName := range nameList.AllAny_name() {
				prc, ok := anyName.(antlr.ParserRuleContext)
				if !ok {
					continue
				}
				name := strings.TrimSpace(ctxText(tokens, prc))
				result.DDLActions = append(result.DDLActions, DDLAction{
					Type:       DDLDropIndex,
					ObjectName: name,
					Flags:      copyFlags(flags),
				})
			}
		}
		return nil
	}

	// DROP object_type_any_name ... (TABLE, INDEX, VIEW, etc.)
	if objType := ctx.Object_type_any_name(); objType != nil {
		if nameList := ctx.Any_name_list_(); nameList != nil {
			switch {
			case objType.TABLE() != nil:
				for _, anyName := range nameList.AllAny_name() {
					prc, ok := anyName.(antlr.ParserRuleContext)
					if !ok {
						continue
					}
					nameText := strings.TrimSpace(ctxText(tokens, prc))
					schema, tableName := splitQualifiedName(nameText)
					result.DDLActions = append(result.DDLActions, DDLAction{
						Type:       DDLDropTable,
						ObjectName: nameText,
						Flags:      copyFlags(flags),
					})
					result.Tables = append(result.Tables, TableRef{
						Schema: schema,
						Name:   tableName,
						Type:   TableTypeBase,
						Raw:    nameText,
					})
				}
			case objType.INDEX() != nil:
				for _, anyName := range nameList.AllAny_name() {
					prc, ok := anyName.(antlr.ParserRuleContext)
					if !ok {
						continue
					}
					name := strings.TrimSpace(ctxText(tokens, prc))
					result.DDLActions = append(result.DDLActions, DDLAction{
						Type:       DDLDropIndex,
						ObjectName: name,
						Flags:      copyFlags(flags),
					})
				}
			}
		}
	}
	return nil
}

// populateAlterTable handles ALTER TABLE with ADD/DROP/ALTER column sub-commands.
func populateAlterTable(result *ParsedQuery, ctx gen.IAltertablestmtContext, tokens antlr.TokenStream) error {
	if ctx == nil {
		return fmt.Errorf("alter table statement: %w", ErrNilContext)
	}
	// Only handle ALTER TABLE (not ALTER INDEX/VIEW/SEQUENCE).
	if ctx.TABLE() == nil {
		return nil
	}

	tableName := ""
	if rel := ctx.Relation_expr(); rel != nil {
		if prc, ok := rel.(antlr.ParserRuleContext); ok {
			tableName = strings.TrimSpace(ctxText(tokens, prc))
		}
		schema, name := splitQualifiedName(tableName)
		result.Tables = append(result.Tables, TableRef{
			Schema: schema,
			Name:   name,
			Type:   TableTypeBase,
			Raw:    tableName,
		})
	}

	cmds := ctx.Alter_table_cmds()
	if cmds == nil {
		return nil
	}
	for _, cmd := range cmds.AllAlter_table_cmd() {
		populateAlterTableCmd(result, cmd, tokens, tableName)
	}
	return nil
}

// populateAlterTableCmd processes a single ALTER TABLE sub-command.
func populateAlterTableCmd(result *ParsedQuery, cmd gen.IAlter_table_cmdContext, tokens antlr.TokenStream, tableName string) {
	if cmd == nil {
		return
	}

	var flags []string
	if db := cmd.Drop_behavior_(); db != nil {
		if db.CASCADE() != nil {
			flags = append(flags, "CASCADE")
		} else if db.RESTRICT() != nil {
			flags = append(flags, "RESTRICT")
		}
	}

	switch {
	case cmd.DROP() != nil:
		// DROP COLUMN vs DROP CONSTRAINT
		if cmd.CONSTRAINT() != nil {
			// Skip constraint drops — not column-level DDL.
			return
		}
		colName := extractAlterCmdColumnName(cmd, tokens)
		if colName == "" {
			return
		}
		if cmd.IF_P() != nil && cmd.EXISTS() != nil {
			flags = append(flags, "IF_EXISTS")
		}
		result.DDLActions = append(result.DDLActions, DDLAction{
			Type:       DDLDropColumn,
			ObjectName: tableName,
			Columns:    []string{colName},
			Flags:      flags,
		})

	case cmd.ADD_P() != nil:
		if cmd.CONSTRAINT() != nil || cmd.Tableconstraint() != nil {
			// Skip ADD CONSTRAINT.
			return
		}
		colName := ""
		if colDef := cmd.ColumnDef(); colDef != nil {
			if colDef.Colid() != nil {
				if prc, ok := colDef.Colid().(antlr.ParserRuleContext); ok {
					colName = strings.TrimSpace(ctxText(tokens, prc))
				}
			}
		}
		if colName == "" {
			return
		}
		addFlags := copyFlags(flags)
		addFlags = append(addFlags, "ADD_COLUMN")
		if cmd.IF_P() != nil && cmd.NOT() != nil && cmd.EXISTS() != nil {
			addFlags = append(addFlags, "IF_NOT_EXISTS")
		}
		result.DDLActions = append(result.DDLActions, DDLAction{
			Type:       DDLAlterTable,
			ObjectName: tableName,
			Columns:    []string{colName},
			Flags:      addFlags,
		})

	case cmd.ALTER() != nil:
		colName := extractAlterCmdColumnName(cmd, tokens)
		if colName == "" {
			return
		}
		alterFlags := copyFlags(flags)
		alterFlags = append(alterFlags, "ALTER_COLUMN")
		result.DDLActions = append(result.DDLActions, DDLAction{
			Type:       DDLAlterTable,
			ObjectName: tableName,
			Columns:    []string{colName},
			Flags:      alterFlags,
		})

	default:
		// Other sub-commands (OWNER TO, SET, etc.) — generic ALTER_TABLE action.
		result.DDLActions = append(result.DDLActions, DDLAction{
			Type:       DDLAlterTable,
			ObjectName: tableName,
			Flags:      flags,
		})
	}
}

// extractAlterCmdColumnName extracts the column name from an ALTER TABLE sub-command.
func extractAlterCmdColumnName(cmd gen.IAlter_table_cmdContext, tokens antlr.TokenStream) string {
	// The column name is usually the first Colid child.
	colids := cmd.AllColid()
	if len(colids) > 0 {
		if prc, ok := colids[0].(antlr.ParserRuleContext); ok {
			return strings.TrimSpace(ctxText(tokens, prc))
		}
	}
	return ""
}

// populateCreateIndex handles CREATE [UNIQUE] INDEX [CONCURRENTLY] ... ON table.
func populateCreateIndex(result *ParsedQuery, ctx gen.IIndexstmtContext, tokens antlr.TokenStream) error {
	if ctx == nil {
		return fmt.Errorf("create index statement: %w", ErrNilContext)
	}

	indexName := ""
	if idx := ctx.Index_name_(); idx != nil {
		if idx.Name() != nil {
			if prc, ok := idx.Name().(antlr.ParserRuleContext); ok {
				indexName = strings.TrimSpace(ctxText(tokens, prc))
			}
		}
	}
	if indexName == "" && ctx.Name() != nil {
		if prc, ok := ctx.Name().(antlr.ParserRuleContext); ok {
			indexName = strings.TrimSpace(ctxText(tokens, prc))
		}
	}

	tableName := ""
	if rel := ctx.Relation_expr(); rel != nil {
		if prc, ok := rel.(antlr.ParserRuleContext); ok {
			tableName = strings.TrimSpace(ctxText(tokens, prc))
		}
		schema, name := splitQualifiedName(tableName)
		result.Tables = append(result.Tables, TableRef{
			Schema: schema,
			Name:   name,
			Type:   TableTypeBase,
			Raw:    tableName,
		})
	}

	var columns []string
	if params := ctx.Index_params(); params != nil {
		for _, elem := range params.AllIndex_elem() {
			prc, ok := elem.(antlr.ParserRuleContext)
			if !ok {
				continue
			}
			text := strings.TrimSpace(ctxText(tokens, prc))
			if text != "" {
				columns = append(columns, text)
			}
		}
	}

	var flags []string
	if ctx.Concurrently_() != nil {
		flags = append(flags, "CONCURRENTLY")
	}
	if ctx.Unique_() != nil {
		flags = append(flags, "UNIQUE")
	}
	if ctx.IF_P() != nil && ctx.NOT() != nil && ctx.EXISTS() != nil {
		flags = append(flags, "IF_NOT_EXISTS")
	}

	indexType := ""
	if amc := ctx.Access_method_clause(); amc != nil {
		if amc.Name() != nil {
			if prc, ok := amc.Name().(antlr.ParserRuleContext); ok {
				indexType = strings.TrimSpace(ctxText(tokens, prc))
			}
		}
	}

	action := DDLAction{
		Type:       DDLCreateIndex,
		ObjectName: indexName,
		Columns:    columns,
		Flags:      flags,
		IndexType:  indexType,
	}
	result.DDLActions = append(result.DDLActions, action)
	return nil
}

// populateTruncate handles TRUNCATE [TABLE] table [, ...] [CASCADE|RESTRICT].
func populateTruncate(result *ParsedQuery, ctx gen.ITruncatestmtContext, tokens antlr.TokenStream) error {
	if ctx == nil {
		return fmt.Errorf("truncate statement: %w", ErrNilContext)
	}

	var flags []string
	if rs := ctx.Restart_seqs_(); rs != nil {
		if rs.RESTART() != nil {
			flags = append(flags, "RESTART_IDENTITY")
		} else if rs.CONTINUE_P() != nil {
			flags = append(flags, "CONTINUE_IDENTITY")
		}
	}
	if db := ctx.Drop_behavior_(); db != nil {
		if db.CASCADE() != nil {
			flags = append(flags, "CASCADE")
		} else if db.RESTRICT() != nil {
			flags = append(flags, "RESTRICT")
		}
	}

	if relList := ctx.Relation_expr_list(); relList != nil {
		for _, rel := range relList.AllRelation_expr() {
			prc, ok := rel.(antlr.ParserRuleContext)
			if !ok {
				continue
			}
			nameText := strings.TrimSpace(ctxText(tokens, prc))
			schema, name := splitQualifiedName(nameText)
			result.DDLActions = append(result.DDLActions, DDLAction{
				Type:       DDLTruncate,
				ObjectName: nameText,
				Flags:      copyFlags(flags),
			})
			result.Tables = append(result.Tables, TableRef{
				Schema: schema,
				Name:   name,
				Type:   TableTypeBase,
				Raw:    nameText,
			})
		}
	}
	return nil
}

// copyFlags returns a copy of the flags slice to avoid shared backing arrays.
func copyFlags(flags []string) []string {
	if len(flags) == 0 {
		return nil
	}
	out := make([]string, len(flags))
	copy(out, flags)
	return out
}

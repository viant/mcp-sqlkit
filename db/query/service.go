package query

import (
	"context"
	"database/sql"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/sqlparser"
	"github.com/viant/sqlx/io"
	"github.com/viant/sqlx/io/read"
	text "github.com/viant/tagly/format/text"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Input struct {
	Query      string
	Connector  string
	Parameters []interface{}
}

type Output struct {
	Data      []interface{} `json:",omitempty"`
	Status    string        `json:"status"`
	Error     string        `json:",omitempty"`
	Connector string        `json:",omitempty"`
}

type Service struct {
	connectors *connector.Service
	operation  client.Operations
	cache      *recordTypeCache
}

func (r *Service) Query(ctx context.Context, input *Input) *Output {
	output := &Output{Status: "ok"}
	err := r.query(ctx, input, output)
	if err != nil {
		output.Error = err.Error()
		output.Status = "error"
	}
	output.Connector = input.Connector
	return output
}

func (r *Service) query(ctx context.Context, input *Input, output *Output) error {
	con, err := r.connectors.Connection(ctx, input.Connector)
	if err != nil {
		return err
	}
	input.Connector = con.Name
	db, err := con.Db(ctx)
	if err != nil {
		return err
	}
	recordType, err := r.recordType(ctx, input, db)
	if err != nil {
		return err
	}
	newRecord := func() interface{} {
		return reflect.New(recordType).Interface()
	}
	reader, err := read.New(ctx, db, input.Query, newRecord)
	if err != nil {
		return err
	}

	return reader.QueryAll(ctx, func(row interface{}) error {
		output.Data = append(output.Data, materializeRow(row))
		return nil
	}, input.Parameters...)
}

func (r *Service) recordType(ctx context.Context, input *Input, db *sql.DB) (reflect.Type, error) {
	// -----------------------------------------------------------------------------------------------------------------
	// Prepare cache key – ensure that semantically equivalent projection lists generate the same key.
	// Include auth namespace to avoid cross-user cache bleed when running in auth mode.
	namespace := "default"
	if ns, err := r.connectors.Namespace(ctx); err == nil && ns != "" {
		namespace = ns
	}
	cacheKey := namespace + "|" + input.Connector + "|" + input.Query
	lcQuery := strings.ToLower(input.Query)
	if strings.Contains(lcQuery, "where ") || strings.Contains(lcQuery, "limit ") || strings.Contains(lcQuery, "order ") {
		if parsed, _ := sqlparser.ParseQuery(input.Query); parsed != nil {
			if parsed.From.X != nil {
				cacheKey = namespace + "|" + input.Connector + "|" + sqlparser.Stringify(parsed.From.X)
			}
			for idx, column := range parsed.List {
				switch {
				case column.Expr != nil:
					cacheKey += sqlparser.Stringify(column.Expr)
				case column.Alias != "":
					cacheKey += column.Alias
				default:
					cacheKey += "col" + strconv.Itoa(idx)
				}
				cacheKey += column.Alias + ","
			}
		}
	}
	// Attempt to retrieve previously computed record type from the LRU cache.
	var recordType reflect.Type
	if cached, ok := r.cache.Get(cacheKey); ok {
		recordType = cached
	} else {
		// Cache miss – detect columns and construct an anonymous struct type.
		columns, err := io.DetectColumns(ctx, db, input.Query, input.Parameters...)
		if err != nil {
			return nil, err
		}
		var (
			fields    []reflect.StructField
			usedNames = make(map[string]bool, len(columns))
		)
		for idx, column := range columns {
			name := column.Name
			if name == "" {
				name = "c" + strconv.Itoa(idx)
			}
			columnCase := text.DetectCaseFormat(name)
			fieldName := columnCase.To(text.CaseFormatUpperCamel).Format(name)

			// Ensure the generated struct field name is a valid Go identifier that satisfies
			// reflect.StructField requirements (exported, unique and non-empty).
			// 1. Identifier must start with an upper-case letter so it is exported – many downstream
			//    libraries (including sqlx) rely on being able to set the field via reflection.
			// 2. It cannot start with a digit and may only contain letters, digits and underscore.
			// 3. Field names within a single struct type must be unique.
			if fieldName == "" {
				fieldName = "X"
			}

			// Replace invalid characters and make sure the first character is an upper-case letter.
			fieldName = sanitizeIdentifier(fieldName)

			// Resolve naming collisions that might arise after sanitisation (for example when two
			// columns differ only by characters that have been stripped).
			for usedNames[fieldName] {
				fieldName += "_"
			}
			usedNames[fieldName] = true
			scanType := column.ScanType()
			if scanType.Kind() != reflect.Pointer && (column.Nullable == "1" || column.Nullable == "true") {
				scanType = reflect.PointerTo(scanType)
			}
			field := reflect.StructField{Name: fieldName, Tag: reflect.StructTag(`sqlx:"` + name + `"`), Type: scanType}
			fields = append(fields, field)
		}
		recordType = reflect.StructOf(fields)
		// Store for future reuse.
		r.cache.Put(cacheKey, recordType)
	}
	return recordType, nil
}

func New(services *connector.Service) *Service {
	return &Service{connectors: services, cache: newRecordTypeCache(10)}
}

// sanitizeIdentifier converts s to a string that is a valid exported Go identifier.
// It guarantees that:
//   - The first rune is an upper-case letter (so the field is exported)
//   - Subsequent runes are limited to letters, digits or underscore.
//
// When the first rune is not a letter, the prefix "X" is added.
func sanitizeIdentifier(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) {
				b.WriteRune('X')
			}
			r = unicode.ToUpper(r)
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "X"
	}
	return b.String()
}

var timeType = reflect.TypeOf(time.Time{})

func materializeRow(row interface{}) interface{} {
	if row == nil {
		return nil
	}

	value := reflect.ValueOf(row)
	return materializeValue(value)
}

func materializeValue(value reflect.Value) interface{} {
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return materializeValue(value.Elem())
	case reflect.String:
		return strings.Clone(value.String())
	case reflect.Slice:
		if value.IsNil() {
			return nil
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return append([]byte(nil), value.Bytes()...)
		}
		result := make([]interface{}, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = materializeValue(value.Index(i))
		}
		return result
	case reflect.Array:
		result := make([]interface{}, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = materializeValue(value.Index(i))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		if value.Type().Key().Kind() != reflect.String {
			return value.Interface()
		}
		result := make(map[string]interface{}, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			result[strings.Clone(iter.Key().String())] = materializeValue(iter.Value())
		}
		return result
	case reflect.Struct:
		if value.Type() == timeType {
			return value.Interface()
		}
		return materializeStruct(value)
	default:
		return value.Interface()
	}
}

func materializeStruct(value reflect.Value) map[string]interface{} {
	result := make(map[string]interface{}, value.NumField())
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		result[fieldOutputName(field)] = materializeValue(value.Field(i))
	}
	return result
}

func fieldOutputName(field reflect.StructField) string {
	if sqlxName := tagName(field.Tag.Get("sqlx")); sqlxName != "" {
		return sqlxName
	}
	if jsonName := tagName(field.Tag.Get("json")); jsonName != "" {
		return jsonName
	}
	return field.Name
}

func tagName(tag string) string {
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return ""
	}
	return name
}

package apidoc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// JSON Schema for Go types, as encoding/json writes them.
//
// A named struct is described once, under components, and referred to
// wherever it is used: so a shape shared by many routes (a node, a page of
// them) reads the same everywhere. One of anahita-libs is named as it is
// ("NodeItem"); one of a service is named with its module ("hashtag.HashtagNodeItem"),
// since every service has a package called responses. A generic struct, and
// one declared inside a function, are written out where they are used.

type schemas struct {
	src        *sources
	components map[string]any
	// Which type each component name stands for, so that two types never
	// share one.
	names map[string]reflect.Type
}

func newSchemas(src *sources) *schemas {
	return &schemas{src: src, components: map[string]any{}, names: map[string]reflect.Type{}}
}

var (
	fileType      = reflect.TypeOf(File{})
	timeType      = reflect.TypeOf(time.Time{})
	rawType       = reflect.TypeOf(json.RawMessage{})
	marshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
)

// of is the schema of a value of type t.
func (s *schemas) of(t reflect.Type) (map[string]any, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch {
	case t == fileType:
		return map[string]any{"type": "string", "format": "binary"}, nil
	case t == timeType:
		return map[string]any{"type": "string", "format": "date-time"}, nil
	case t == rawType:
		// Any JSON at all.
		return map[string]any{}, nil
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8:
		return map[string]any{"type": "string", "contentEncoding": "base64"}, nil
	case t.Kind() != reflect.Struct && (t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType)):
		// Written by its own MarshalJSON, which reflection cannot see into.
		return map[string]any{}, nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int8, reflect.Int16, reflect.Int32:
		return map[string]any{"type": "integer", "format": "int32"}, nil
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}, nil
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return map[string]any{"type": "integer", "format": "int32", "minimum": 0}, nil
	case reflect.Uint, reflect.Uint64:
		return map[string]any{"type": "integer", "format": "int64", "minimum": 0}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Interface:
		return map[string]any{}, nil
	case reflect.Slice, reflect.Array:
		items, err := s.of(t.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case reflect.Map:
		values, err := s.of(t.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "object", "additionalProperties": values}, nil
	case reflect.Struct:
		return s.object(t)
	}
	return nil, fmt.Errorf("apidoc: cannot describe a %s", t)
}

// object is a struct's schema: a reference to it among the components when
// it can be named, written out otherwise.
func (s *schemas) object(t reflect.Type) (map[string]any, error) {
	name := componentName(t)
	if name == "" {
		return s.properties(t)
	}
	ref := map[string]any{"$ref": "#/components/schemas/" + name}
	if known, ok := s.names[name]; ok {
		if known != t {
			return nil, fmt.Errorf("apidoc: %s and %s would both be described as %q", known, t, name)
		}
		return ref, nil
	}
	// Named before it is built, so that a type that holds itself refers to
	// itself and is not written out without end.
	s.names[name] = t
	schema, err := s.properties(t)
	if err != nil {
		return nil, err
	}
	s.components[name] = schema
	return ref, nil
}

// componentName is what a struct is called among the components, or "" for
// one that is written out where it is used.
func componentName(t reflect.Type) string {
	if t.Name() == "" || t.PkgPath() == "" || strings.Contains(t.Name(), "[") {
		return ""
	}
	if strings.HasPrefix(t.PkgPath(), "anahita-libs/") {
		return t.Name()
	}
	module := strings.SplitN(t.PkgPath(), "/", 2)[0]
	return module + "." + t.Name()
}

// properties is a struct written out: each field as encoding/json writes it.
func (s *schemas) properties(t reflect.Type) (map[string]any, error) {
	schema := map[string]any{"type": "object"}
	props := map[string]any{}
	var required []string

	if comments, err := s.src.comments(t.PkgPath(), t.Name()); err != nil {
		return nil, err
	} else if comments != nil && comments.doc != "" && componentName(t) != "" {
		schema["description"] = comments.doc
	}

	if err := s.fields(t, props, &required); err != nil {
		return nil, err
	}
	schema["properties"] = props
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema, nil
}

func (s *schemas) fields(t reflect.Type, props map[string]any, required *[]string) error {
	comments, err := s.src.comments(t.PkgPath(), t.Name())
	if err != nil {
		return err
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, opts := jsonName(f)
		if name == "-" {
			continue
		}

		// An embedded struct with no name of its own: its fields are this
		// one's, as encoding/json writes them.
		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				if err := s.fields(ft, props, required); err != nil {
					return err
				}
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}

		field, err := s.of(f.Type)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", t, f.Name, err)
		}
		if opts["string"] {
			field = map[string]any{"type": "string"}
		}
		if comments != nil {
			if text := comments.fields[f.Name]; text != "" {
				field = withDescription(field, text)
			}
		}
		// A field of an embedded struct is shadowed by one of the same name
		// outside it, as encoding/json has it: written once.
		_, again := props[name]
		if !opts["omitempty"] && !again {
			*required = append(*required, name)
			if nullable(f.Type) {
				field = withNull(field)
			}
		}
		props[name] = field
	}
	return nil
}

// jsonName is a field's name in JSON and its options, from its tag. ""
// when the tag gives none.
func jsonName(f reflect.StructField) (string, map[string]bool) {
	opts := map[string]bool{}
	tag, ok := f.Tag.Lookup("json")
	if !ok {
		return "", opts
	}
	parts := strings.Split(tag, ",")
	for _, opt := range parts[1:] {
		opts[opt] = true
	}
	return parts[0], opts
}

// nullable says whether encoding/json may write a field of this type as
// null: a pointer, a slice or a map that is nil, or an interface.
func nullable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map:
		return true
	case reflect.Slice:
		return t != rawType
	}
	return false
}

func withDescription(schema map[string]any, text string) map[string]any {
	out := map[string]any{}
	for k, v := range schema {
		out[k] = v
	}
	out["description"] = text
	return out
}

// withNull is a schema that also allows null.
func withNull(schema map[string]any) map[string]any {
	if len(schema) == 0 || (len(schema) == 1 && schema["description"] != nil) {
		return schema
	}
	if ref, ok := schema["$ref"]; ok {
		out := map[string]any{"anyOf": []any{map[string]any{"$ref": ref}, map[string]any{"type": "null"}}}
		if d, ok := schema["description"]; ok {
			out["description"] = d
		}
		return out
	}
	out := map[string]any{}
	for k, v := range schema {
		out[k] = v
	}
	if kind, ok := schema["type"].(string); ok {
		out["type"] = []any{kind, "null"}
	}
	return out
}

// validated adds to a schema what a validate tag says a value may be, and
// answers with the rules that say something JSON Schema cannot.
func validated(schema map[string]any, kind reflect.Kind, tag string) (required bool, other []string) {
	for _, rule := range strings.Split(tag, ",") {
		key, value, _ := strings.Cut(rule, "=")
		switch key {
		case "", "omitempty":
		case "required":
			required = true
		case "min", "max", "len":
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				other = append(other, rule)
				continue
			}
			bound(schema, kind, key, n)
		case "gt", "gte", "lt", "lte":
			n, err := strconv.ParseFloat(value, 64)
			if err != nil || !isNumber(kind) {
				other = append(other, rule)
				continue
			}
			name := map[string]string{"gt": "exclusiveMinimum", "gte": "minimum", "lt": "exclusiveMaximum", "lte": "maximum"}[key]
			schema[name] = number(n)
		case "oneof":
			var enum []any
			for _, each := range strings.Fields(value) {
				if isNumber(kind) {
					if n, err := strconv.ParseFloat(each, 64); err == nil {
						enum = append(enum, number(n))
						continue
					}
				}
				enum = append(enum, each)
			}
			schema["enum"] = enum
		case "email":
			schema["format"] = "email"
		case "url", "uri", "http_url":
			schema["format"] = "uri"
		case "uuid", "uuid4":
			schema["format"] = "uuid"
		case "number", "numeric":
			if !isNumber(kind) {
				other = append(other, rule)
			}
		default:
			other = append(other, rule)
		}
	}
	return required, other
}

func bound(schema map[string]any, kind reflect.Kind, key string, n float64) {
	var lower, upper string
	switch {
	case kind == reflect.String:
		lower, upper = "minLength", "maxLength"
	case kind == reflect.Slice || kind == reflect.Array || kind == reflect.Map:
		lower, upper = "minItems", "maxItems"
	case isNumber(kind):
		lower, upper = "minimum", "maximum"
	default:
		return
	}
	switch key {
	case "min":
		schema[lower] = number(n)
	case "max":
		schema[upper] = number(n)
	case "len":
		schema[lower], schema[upper] = number(n), number(n)
	}
}

func isNumber(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// number is a bound as JSON writes it: a whole number without a point.
func number(n float64) any {
	if n == float64(int64(n)) {
		return int64(n)
	}
	return n
}

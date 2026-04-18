package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <schema.avsc> <payload.json>\n", os.Args[0])
		os.Exit(2)
	}

	schemaBytes, err := os.ReadFile(os.Args[1])
	if err != nil {
		exitErr(fmt.Errorf("read schema: %w", err))
	}
	payloadBytes, err := os.ReadFile(os.Args[2])
	if err != nil {
		exitErr(fmt.Errorf("read payload: %w", err))
	}

	var schema any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		exitErr(fmt.Errorf("parse schema: %w", err))
	}

	var payload any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		exitErr(fmt.Errorf("parse payload: %w", err))
	}

	if err := validate(schema, payload, "$"); err != nil {
		exitErr(err)
	}

	fmt.Println("ok")
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func validate(schema any, value any, path string) error {
	switch node := schema.(type) {
	case string:
		return validatePrimitive(node, value, path)
	case []any:
		return validateUnion(node, value, path)
	case map[string]any:
		return validateTyped(node, value, path)
	default:
		return fmt.Errorf("%s: unsupported schema node %T", path, schema)
	}
}

func validatePrimitive(schemaType string, value any, path string) error {
	switch schemaType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string", path)
		}
		return nil
	case "null":
		if value != nil {
			return fmt.Errorf("%s: expected null", path)
		}
		return nil
	default:
		return fmt.Errorf("%s: unsupported primitive type %q", path, schemaType)
	}
}

func validateUnion(options []any, value any, path string) error {
	var errs []error
	for _, option := range options {
		if err := validate(option, value, path); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}
	return fmt.Errorf("%s: no union branch matched: %w", path, errors.Join(errs...))
}

func validateTyped(node map[string]any, value any, path string) error {
	kind, ok := node["type"]
	if !ok {
		return fmt.Errorf("%s: schema node missing type", path)
	}

	switch typed := kind.(type) {
	case string:
		switch typed {
		case "record":
			return validateRecord(node, value, path)
		case "array":
			return validateArray(node, value, path)
		case "map":
			return validateMap(node, value, path)
		case "string", "null":
			return validatePrimitive(typed, value, path)
		default:
			return fmt.Errorf("%s: unsupported complex type %q", path, typed)
		}
	case []any:
		return validateUnion(typed, value, path)
	case map[string]any:
		return validateTyped(typed, value, path)
	default:
		return fmt.Errorf("%s: unsupported typed schema node %T", path, kind)
	}
}

func validateRecord(node map[string]any, value any, path string) error {
	record, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: expected object", path)
	}

	rawFields, ok := node["fields"].([]any)
	if !ok {
		return fmt.Errorf("%s: record fields missing or invalid", path)
	}

	allowed := make(map[string]struct{}, len(rawFields))
	for _, rawField := range rawFields {
		field, ok := rawField.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: invalid field definition", path)
		}
		name, _ := field["name"].(string)
		if name == "" {
			return fmt.Errorf("%s: field without name", path)
		}
		allowed[name] = struct{}{}

		fieldPath := path + "." + name
		fieldValue, exists := record[name]
		if !exists {
			if _, hasDefault := field["default"]; hasDefault {
				continue
			}
			if union, ok := field["type"].([]any); ok && unionAllowsNull(union) {
				continue
			}
			return fmt.Errorf("%s: missing required field", fieldPath)
		}

		if err := validate(field["type"], fieldValue, fieldPath); err != nil {
			return err
		}
	}

	for key := range record {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%s.%s: unexpected field", path, key)
		}
	}

	return nil
}

func validateArray(node map[string]any, value any, path string) error {
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s: expected array", path)
	}
	itemSchema, ok := node["items"]
	if !ok {
		return fmt.Errorf("%s: array items missing", path)
	}
	for idx, item := range items {
		if err := validate(itemSchema, item, fmt.Sprintf("%s[%d]", path, idx)); err != nil {
			return err
		}
	}
	return nil
}

func validateMap(node map[string]any, value any, path string) error {
	entries, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: expected object map", path)
	}
	valueSchema, ok := node["values"]
	if !ok {
		return fmt.Errorf("%s: map values schema missing", path)
	}
	for key, entry := range entries {
		if err := validate(valueSchema, entry, path+"."+key); err != nil {
			return err
		}
	}
	return nil
}

func unionAllowsNull(options []any) bool {
	for _, option := range options {
		if primitive, ok := option.(string); ok && primitive == "null" {
			return true
		}
	}
	return false
}

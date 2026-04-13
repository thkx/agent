package tool

import (
	"encoding/json"
	"fmt"
)

type Schema struct {
	Type       string              `json:"type,omitempty"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

func (s Schema) JSON() map[string]any {
	var out map[string]any
	b, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func (s Schema) Validate(input any) error {
	if s.Type == "" {
		return nil
	}

	return validateValue("input", propertyFromSchema(s), input)
}

func matchesType(kind string, value any) bool {
	switch kind {
	case "", "any":
		return true
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, float32, float64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}

func propertyFromSchema(s Schema) Property {
	return Property{
		Type:       s.Type,
		Properties: cloneProperties(s.Properties),
		Required:   append([]string(nil), s.Required...),
	}
}

func validateValue(path string, prop Property, input any) error {
	if prop.Type == "" {
		return nil
	}

	switch prop.Type {
	case "object":
		if input == nil {
			if len(prop.Required) == 0 {
				return nil
			}
			return fmt.Errorf("%s is required", path)
		}

		values, ok := input.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}

		for _, name := range prop.Required {
			value, exists := values[name]
			if !exists || value == nil || value == "" {
				return fmt.Errorf("%s.%s is required", path, name)
			}
		}

		for name, nested := range prop.Properties {
			value, exists := values[name]
			if !exists || value == nil {
				continue
			}
			if err := validateValue(path+"."+name, nested, value); err != nil {
				return err
			}
		}

		return nil
	case "array":
		values, ok := toAnySlice(input)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if prop.Items == nil {
			return nil
		}
		for i, item := range values {
			if err := validateValue(fmt.Sprintf("%s[%d]", path, i), *prop.Items, item); err != nil {
				return err
			}
		}
		return nil
	default:
		if !matchesType(prop.Type, input) {
			return fmt.Errorf("%s must be %s", path, prop.Type)
		}
		if len(prop.Enum) > 0 && !matchesEnum(prop.Enum, input) {
			return fmt.Errorf("%s must be one of %v", path, prop.Enum)
		}
		return nil
	}
}

func toAnySlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []string:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = v
		}
		return out, true
	case []int:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = v
		}
		return out, true
	case []float64:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = v
		}
		return out, true
	case []bool:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = v
		}
		return out, true
	default:
		return nil, false
	}
}

func matchesEnum(options []any, value any) bool {
	for _, option := range options {
		if fmt.Sprintf("%v", option) == fmt.Sprintf("%v", value) {
			return true
		}
	}
	return false
}

func cloneProperties(src map[string]Property) map[string]Property {
	if src == nil {
		return nil
	}

	dst := make(map[string]Property, len(src))
	for name, prop := range src {
		dst[name] = cloneProperty(prop)
	}
	return dst
}

func cloneProperty(prop Property) Property {
	cloned := Property{
		Type:        prop.Type,
		Description: prop.Description,
		Properties:  cloneProperties(prop.Properties),
		Required:    append([]string(nil), prop.Required...),
		Enum:        append([]any(nil), prop.Enum...),
	}
	if prop.Items != nil {
		item := cloneProperty(*prop.Items)
		cloned.Items = &item
	}
	return cloned
}

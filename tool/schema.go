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

	switch s.Type {
	case "object":
		if input == nil {
			if len(s.Required) == 0 {
				return nil
			}
			return fmt.Errorf("input is required")
		}

		values, ok := input.(map[string]any)
		if !ok {
			return fmt.Errorf("input must be an object")
		}

		for _, name := range s.Required {
			value, exists := values[name]
			if !exists || value == nil || value == "" {
				return fmt.Errorf("missing required field %q", name)
			}
		}

		for name, prop := range s.Properties {
			value, exists := values[name]
			if !exists || value == nil {
				continue
			}
			if !matchesType(prop.Type, value) {
				return fmt.Errorf("field %q must be %s", name, prop.Type)
			}
		}

		return nil
	default:
		if !matchesType(s.Type, input) {
			return fmt.Errorf("input must be %s", s.Type)
		}
		return nil
	}
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

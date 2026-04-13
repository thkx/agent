package tool

import "testing"

func TestSchemaValidateNestedObjectAndArray(t *testing.T) {
	t.Parallel()

	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"config": {
				Type: "object",
				Properties: map[string]Property{
					"mode": {
						Type: "string",
						Enum: []any{"fast", "safe"},
					},
				},
				Required: []string{"mode"},
			},
			"symbols": {
				Type: "array",
				Items: &Property{
					Type: "string",
				},
			},
		},
		Required: []string{"config", "symbols"},
	}

	if err := schema.Validate(map[string]any{
		"config": map[string]any{
			"mode": "fast",
		},
		"symbols": []any{"BTC", "ETH"},
	}); err != nil {
		t.Fatalf("expected nested schema to validate, got %v", err)
	}
}

func TestSchemaValidateReportsNestedErrors(t *testing.T) {
	t.Parallel()

	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"config": {
				Type: "object",
				Properties: map[string]Property{
					"mode": {
						Type: "string",
						Enum: []any{"fast", "safe"},
					},
				},
				Required: []string{"mode"},
			},
			"symbols": {
				Type: "array",
				Items: &Property{
					Type: "string",
				},
			},
		},
		Required: []string{"config", "symbols"},
	}

	if err := schema.Validate(map[string]any{
		"config":  map[string]any{},
		"symbols": []any{"BTC", 1},
	}); err == nil {
		t.Fatal("expected nested validation error")
	}

	if err := schema.Validate(map[string]any{
		"config": map[string]any{
			"mode": "turbo",
		},
		"symbols": []any{"BTC"},
	}); err == nil {
		t.Fatal("expected enum validation error")
	}
}

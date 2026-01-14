package config

import "testing"

func TestMergeMapReplacesArrays(t *testing.T) {
	dst := map[string]any{
		"list": []any{1, 2},
		"nested": map[string]any{
			"a": 1,
		},
	}
	src := map[string]any{
		"list": []any{3},
		"nested": map[string]any{
			"b": 2,
		},
	}

	mergeMap(dst, src)

	list := dst["list"].([]any)
	if len(list) != 1 || list[0].(int) != 3 {
		t.Fatalf("expected list replacement, got %#v", list)
	}

	nested := dst["nested"].(map[string]any)
	if nested["a"].(int) != 1 || nested["b"].(int) != 2 {
		t.Fatalf("expected nested map merge, got %#v", nested)
	}
}

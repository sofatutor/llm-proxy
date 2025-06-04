package eventtransformer

import (
	"reflect"
	"testing"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"CamelCase", "camel_case"},
		{"PascalCase", "pascal_case"},
		{"kebab-case", "kebab_case"},
		{"snake_case", "snake_case"},
		{"HTTPServer", "http_server"},
		{"already_snake_case", "already_snake_case"},
		{"with-dash_and_Underscore", "with_dash_and_underscore"},
		{"A", "a"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ToSnakeCase(tt.in); got != tt.want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToSnakeCaseMap(t *testing.T) {
	in := map[string]interface{}{
		"CamelCase":  1,
		"PascalCase": map[string]interface{}{"InnerKey": 2},
		"kebab-case": []interface{}{
			map[string]interface{}{"NestedKey": 3},
			4,
		},
	}
	want := map[string]interface{}{
		"camel_case":  1,
		"pascal_case": map[string]interface{}{"inner_key": 2},
		"kebab_case": []interface{}{
			map[string]interface{}{"nested_key": 3},
			4,
		},
	}
	got := ToSnakeCaseMap(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToSnakeCaseMap() = %#v, want %#v", got, want)
	}
}

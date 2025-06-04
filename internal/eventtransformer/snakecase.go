package eventtransformer

import (
	"strings"
	"unicode"
)

// ToSnakeCaseMap recursively converts all map keys to snake_case, replacing dashes and handling consecutive uppercase letters
func ToSnakeCaseMap(m map[string]interface{}) map[string]interface{} {
	snake := make(map[string]interface{}, len(m))
	for k, v := range m {
		sk := ToSnakeCase(k)
		switch vv := v.(type) {
		case map[string]interface{}:
			snake[sk] = ToSnakeCaseMap(vv)
		case []interface{}:
			arr := make([]interface{}, len(vv))
			for i, elem := range vv {
				if mm, ok := elem.(map[string]interface{}); ok {
					arr[i] = ToSnakeCaseMap(mm)
				} else {
					arr[i] = elem
				}
			}
			snake[sk] = arr
		default:
			snake[sk] = v
		}
	}
	return snake
}

// ToSnakeCase converts a string from CamelCase, PascalCase, or kebab-case to snake_case
func ToSnakeCase(s string) string {
	// Replace dashes with underscores
	s = strings.ReplaceAll(s, "-", "_")
	var out []rune
	var prevLower, prevUnderscore bool
	for i, r := range s {
		if r == '_' {
			out = append(out, r)
			prevLower, prevUnderscore = false, true
			continue
		}
		if unicode.IsUpper(r) {
			if (i > 0 && prevLower) || (i > 0 && !prevUnderscore && i+1 < len(s) && unicode.IsLower(rune(s[i+1]))) {
				out = append(out, '_')
			}
			out = append(out, unicode.ToLower(r))
			prevLower, prevUnderscore = false, false
		} else {
			out = append(out, r)
			prevLower, prevUnderscore = true, false
		}
	}
	return string(out)
}

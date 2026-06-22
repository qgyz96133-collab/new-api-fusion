package translate

import "fmt"

// UnsupportedSchemaConstraints lists JSON Schema keywords not supported by Gemini API.
// Ported from 9Router: open-sse/translator/helpers/geminiHelper.js
var UnsupportedSchemaConstraints = []string{
	// Basic constraints (not supported by Gemini API)
	"minLength", "maxLength", "exclusiveMinimum", "exclusiveMaximum",
	"pattern", "minItems", "maxItems", "format",
	// Claude rejects these in VALIDATED mode
	"default", "examples",
	// JSON Schema meta keywords
	"$schema", "$defs", "definitions", "const", "$ref", "$comment",
	// Object validation keywords (not supported)
	"additionalProperties", "propertyNames", "patternProperties", "enumDescriptions",
	// Complex schema keywords (handled by flattenAnyOfOneOf/mergeAllOf)
	"anyOf", "oneOf", "allOf", "not",
	// Dependency keywords (not supported)
	"dependencies", "dependentSchemas", "dependentRequired",
	// Other unsupported keywords
	"title", "if", "then", "else", "contentMediaType", "contentEncoding",
	// UI/Styling properties (from Cursor tools — NOT JSON Schema standard)
	"cornerRadius", "fillColor", "fontFamily", "fontSize", "fontWeight",
	"gap", "padding", "strokeColor", "strokeThickness", "textColor",
}

// unsupportedSet for O(1) lookup
var unsupportedSet map[string]bool

func init() {
	unsupportedSet = make(map[string]bool, len(UnsupportedSchemaConstraints))
	for _, k := range UnsupportedSchemaConstraints {
		unsupportedSet[k] = true
	}
}

// CleanGeminiSchema is the main entry point — runs the full 5-phase pipeline
// to make a JSON Schema compatible with Gemini/Antigravity API.
func CleanGeminiSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}

	// Phase 1: Convert and prepare
	convertConstToEnum(schema)
	convertEnumValuesToStrings(schema)

	// Phase 2: Flatten complex structures
	mergeAllOf(schema)
	flattenAnyOfOneOf(schema)
	flattenTypeArrays(schema)

	// Phase 2.5: Infer missing type=object when properties exist
	ensureObjectType(schema)

	// Phase 3: Remove all unsupported keywords at ALL levels
	removeUnsupportedKeywords(schema)

	// Phase 4: Cleanup required fields
	cleanupRequired(schema)

	// Phase 5: Add placeholder for empty object schemas
	addPlaceholders(schema)

	return schema
}

// Phase 1a: Convert const → enum
func convertConstToEnum(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if c, ok := obj["const"]; ok {
		if _, hasEnum := obj["enum"]; !hasEnum {
			obj["enum"] = []interface{}{c}
		}
		delete(obj, "const")
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			convertConstToEnum(child)
		}
	}
}

// Phase 1b: Convert all enum values to strings (Gemini requires string enum + type:"string")
func convertEnumValuesToStrings(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if enumVal, ok := obj["enum"]; ok {
		if arr, ok := enumVal.([]interface{}); ok {
			strArr := make([]interface{}, len(arr))
			for i, v := range arr {
				strArr[i] = toString(v)
			}
			obj["enum"] = strArr
			if _, hasType := obj["type"]; !hasType {
				obj["type"] = "string"
			}
		}
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			convertEnumValuesToStrings(child)
		}
	}
}

// Phase 2a: Merge allOf schemas
func mergeAllOf(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if allOfVal, ok := obj["allOf"]; ok {
		if allOfArr, ok := allOfVal.([]interface{}); ok {
			mergedProps := make(map[string]interface{})
			var mergedRequired []interface{}

			for _, item := range allOfArr {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if props, ok := itemMap["properties"].(map[string]interface{}); ok {
						for k, v := range props {
							mergedProps[k] = v
						}
					}
					if req, ok := itemMap["required"].([]interface{}); ok {
						for _, r := range req {
							if !containsInterface(mergedRequired, r) {
								mergedRequired = append(mergedRequired, r)
							}
						}
					}
				}
			}

			delete(obj, "allOf")
			if len(mergedProps) > 0 {
				existingProps, _ := obj["properties"].(map[string]interface{})
				if existingProps == nil {
					existingProps = make(map[string]interface{})
				}
				for k, v := range mergedProps {
					existingProps[k] = v
				}
				obj["properties"] = existingProps
			}
			if len(mergedRequired) > 0 {
				existingReq, _ := obj["required"].([]interface{})
				obj["required"] = append(existingReq, mergedRequired...)
			}
		}
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			mergeAllOf(child)
		}
	}
}

// Phase 2b: Flatten anyOf/oneOf — select best schema
func flattenAnyOfOneOf(obj map[string]interface{}) {
	if obj == nil {
		return
	}

	flattenKey := func(key string) {
		if val, ok := obj[key]; ok {
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				// Filter out null schemas
				var nonNull []map[string]interface{}
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						if t, _ := m["type"].(string); t != "null" {
							nonNull = append(nonNull, m)
						}
					}
				}
				if len(nonNull) > 0 {
					best := selectBestSchema(nonNull)
					delete(obj, key)
					for k, v := range best {
						obj[k] = v
					}
				}
			}
		}
	}

	flattenKey("anyOf")
	flattenKey("oneOf")

	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			flattenAnyOfOneOf(child)
		}
	}
}

// selectBestSchema picks the schema with highest score:
// object=3, array=2, typed=1, null=0
func selectBestSchema(schemas []map[string]interface{}) map[string]interface{} {
	bestIdx := 0
	bestScore := -1

	for i, item := range schemas {
		score := 0
		t, _ := item["type"].(string)
		_, hasProps := item["properties"]
		_, hasItems := item["items"]

		if t == "object" || hasProps {
			score = 3
		} else if t == "array" || hasItems {
			score = 2
		} else if t != "" && t != "null" {
			score = 1
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return schemas[bestIdx]
}

// Phase 2c: Flatten type arrays (e.g. ["string", "null"] → "string")
func flattenTypeArrays(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if typeVal, ok := obj["type"]; ok {
		if typeArr, ok := typeVal.([]interface{}); ok {
			var nonNull []string
			for _, t := range typeArr {
				if s, ok := t.(string); ok && s != "null" {
					nonNull = append(nonNull, s)
				}
			}
			if len(nonNull) > 0 {
				obj["type"] = nonNull[0]
			} else {
				obj["type"] = "string"
			}
		}
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			flattenTypeArrays(child)
		}
	}
}

// Phase 2.5: Infer type=object when properties exist
func ensureObjectType(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if _, hasProps := obj["properties"]; hasProps {
		if _, hasType := obj["type"]; !hasType {
			obj["type"] = "object"
		}
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			ensureObjectType(child)
		}
	}
}

// Phase 3: Remove all unsupported keywords recursively (including vendor extensions x-*)
func removeUnsupportedKeywords(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	for key, val := range obj {
		if unsupportedSet[key] || (len(key) > 2 && key[:2] == "x-") {
			delete(obj, key)
			continue
		}
		if child, ok := val.(map[string]interface{}); ok {
			removeUnsupportedKeywords(child)
		} else if arr, ok := val.([]interface{}); ok {
			for _, item := range arr {
				if childItem, ok := item.(map[string]interface{}); ok {
					removeUnsupportedKeywords(childItem)
				}
			}
		}
	}
}

// Phase 4: Cleanup required — remove refs to non-existent properties
func cleanupRequired(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if reqVal, ok := obj["required"]; ok {
		if reqArr, ok := reqVal.([]interface{}); ok {
			if props, ok := obj["properties"].(map[string]interface{}); ok {
				var validReq []interface{}
				for _, r := range reqArr {
					if field, ok := r.(string); ok {
						if _, exists := props[field]; exists {
							validReq = append(validReq, field)
						}
					}
				}
				if len(validReq) == 0 {
					delete(obj, "required")
				} else {
					obj["required"] = validReq
				}
			}
		}
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			cleanupRequired(child)
		}
	}
}

// Phase 5: Add placeholder for empty object schemas (Gemini requirement)
func addPlaceholders(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	if t, _ := obj["type"].(string); t == "object" {
		props, _ := obj["properties"].(map[string]interface{})
		if len(props) == 0 {
			obj["properties"] = map[string]interface{}{
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Brief explanation of why you are calling this tool",
				},
			}
			obj["required"] = []interface{}{"reason"}
		}
	}
	for _, v := range obj {
		if child, ok := v.(map[string]interface{}); ok {
			addPlaceholders(child)
		}
	}
}

// --- helpers ---

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func containsInterface(arr []interface{}, val interface{}) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

// CleanToolsSchemas applies CleanGeminiSchema to each tool's function parameters
func CleanToolsSchemas(tools []interface{}) []interface{} {
	for _, tool := range tools {
		if toolMap, ok := tool.(map[string]interface{}); ok {
			if fn, ok := toolMap["function"].(map[string]interface{}); ok {
				if params, ok := fn["parameters"].(map[string]interface{}); ok {
					fn["parameters"] = CleanGeminiSchema(params)
				}
			}
		}
	}
	return tools
}

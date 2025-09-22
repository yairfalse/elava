package aws

import (
	"reflect"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/yairfalse/elava/types"
)

// convertTagsToElava converts ANY AWS tag type to Elava tags using reflection
// ONE function to rule them all - no more duplication!
func (p *RealAWSProvider) convertTagsToElava(tags interface{}) types.Tags {
	result := types.Tags{}

	if tags == nil {
		return result
	}

	// Handle the input based on its type
	v := reflect.ValueOf(tags)

	switch v.Kind() {
	case reflect.Slice:
		// Handle slice of tags (most AWS services)
		for i := 0; i < v.Len(); i++ {
			tag := v.Index(i)
			key, value := extractTagKeyValue(tag.Interface())
			applyTagToResult(&result, key, value)
		}

	case reflect.Map:
		// Handle map[string]string or map[string]*string (Lambda, EKS, etc.)
		for _, mapKey := range v.MapKeys() {
			mapValue := v.MapIndex(mapKey)
			key := mapKey.String()
			value := extractStringValue(mapValue.Interface())
			applyTagToResult(&result, key, value)
		}
	}

	return result
}

// extractTagKeyValue extracts Key and Value fields from any AWS tag struct
func extractTagKeyValue(tag interface{}) (string, string) {
	v := reflect.ValueOf(tag)

	// Handle pointer
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Extract Key and Value fields
	var key, value string

	if keyField := v.FieldByName("Key"); keyField.IsValid() {
		key = extractStringValue(keyField.Interface())
	}

	if valueField := v.FieldByName("Value"); valueField.IsValid() {
		value = extractStringValue(valueField.Interface())
	}

	return key, value
}

// extractStringValue handles *string and string types
func extractStringValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case *string:
		return aws.ToString(val)
	default:
		// Try reflection as last resort
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.String {
			return rv.String()
		}
		return ""
	}
}

// applyTagToResult applies a single tag to the result based on key
// This is the SINGLE SOURCE OF TRUTH for tag mapping logic
func applyTagToResult(result *types.Tags, key, value string) {
	switch key {
	// Elava-specific tags
	case "elava:owner", "ElavaOwner":
		result.ElavaOwner = value
	case "elava:managed", "ElavaManaged":
		result.ElavaManaged = value == "true"
	case "elava:blessed", "ElavaBlessed":
		result.ElavaBlessed = value == "true"

	// Common ownership tags (case variations)
	case "Owner", "owner", "OWNER":
		if result.ElavaOwner == "" {
			result.ElavaOwner = value
		}

	// Environment tags
	case "Environment", "environment", "env", "Env", "ENV":
		if result.Environment == "" {
			result.Environment = value
		}

	// Team tags
	case "Team", "team", "TEAM":
		if result.Team == "" {
			result.Team = value
		}

	// Name tags
	case "Name", "name", "NAME":
		if result.Name == "" {
			result.Name = value
		}

	// Project tags
	case "Project", "project", "PROJECT":
		if result.Project == "" {
			result.Project = value
		}

	// Cost center tags
	case "CostCenter", "cost-center", "costcenter", "cost_center":
		if result.CostCenter == "" {
			result.CostCenter = value
		}
	}
}

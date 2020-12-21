package cloudfoundry

import (
	"fmt"
	"strings"
)

// it would be nice to use validation.MapValueMatch(), except it produces a
// schema.SchemaValidateDiagFunc, which isn't usable with validation.All()
func validateEnvMapEmptyStrings(v interface{}, k string) (warnings []string, errors []error) {
	m := v.(map[string]interface{})

	for key := range m {
		val := m[key]

		s, ok := val.(string)
		if !ok {
			errors = append(errors, fmt.Errorf("%q: map values should be strings", key))
		} else if s == "" {
			// cf client won't persist empty strings to app state
			errors = append(errors, fmt.Errorf("%q: Cannot set environment variables to empty strings", key))
		}
	}

	return warnings, errors
}

// it would be nice to use validation.MapKeyMatch(), except, again it produces a
// schema.SchemaValidateDiagFunc, which isn't usable with validation.All() - on top
// of that, golang's regexp engine doesn't support negation, because hair shirt and
// all that
func validateEnvMapKeysPattern(v interface{}, k string) (warnings []string, errors []error) {
	m := v.(map[string]interface{})

	for key := range m {
		if key == "PORT" {
			errors = append(errors, fmt.Errorf("%q: The PORT environment variable is reserved", key))
		}
		if strings.HasPrefix(key, "VCAP_") {
			errors = append(errors, fmt.Errorf("%q: Environment variables starting with VCAP_ are reserved", key))
		}
	}

	return warnings, errors
}

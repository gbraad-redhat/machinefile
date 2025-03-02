package machinefile

import (
	"strings"
)

func expandVariables(input string, envVars map[string]string) string {
	result := input
	// Match ${VAR} format
	for key, value := range envVars {
		result = strings.ReplaceAll(result, "${"+key+"}", value)
		// Also handle $VAR format
		result = strings.ReplaceAll(result, "$"+key, value)
	}
	return result
}

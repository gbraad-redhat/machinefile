package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Custom flag type that supports a primary name and shorthand
type flagWithShorthand struct {
	name      string
	shorthand string
	value     flag.Value
	usage     string
	isset     bool
}

// Collection of flags with shorthands
var flagsWithShorthands []*flagWithShorthand

// Helper function to create a new flag with shorthand
func newFlagWithShorthand(name, shorthand string, value flag.Value, usage string) *flagWithShorthand {
	f := &flagWithShorthand{
		name:      name,
		shorthand: shorthand,
		value:     value,
		usage:     usage,
	}
	flagsWithShorthands = append(flagsWithShorthands, f)
	return f
}

// Define flag categories for organized help output
type flagCategory struct {
	name  string
	flags []string
}

// Helper function to check if a flag is in a category
func flagInCategory(flagName string, categoryFlags []string) bool {
	for _, f := range categoryFlags {
		if f == flagName {
			return true
		}
	}
	return false
}

// Helper function to print a flag's information
func printFlagHelp(f *flag.Flag) string {
	// For flags with shorthands, show both forms
	for _, fs := range flagsWithShorthands {
		if fs.name == f.Name {
			return fmt.Sprintf("  %-20s %s", fmt.Sprintf("-%s, --%s", fs.shorthand, fs.name), fs.usage)
		}
		// Skip printing shorthand flags separately
		if fs.shorthand == f.Name {
			return ""
		}
	}
	
	// Regular flags
	return fmt.Sprintf("  %-20s %s", fmt.Sprintf("--%s", f.Name), f.Usage)
}

// stringValue is a helper type to satisfy flag.Value interface
type stringValue string

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) String() string {
	return string(*s)
}

// boolValue is a helper type to satisfy flag.Value interface for boolean flags
type boolValue bool

func newBoolValue(b *bool) *boolValue {
	return (*boolValue)(b)
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	*b = boolValue(v)
	return nil
}

func (b *boolValue) String() string {
	return strconv.FormatBool(bool(*b))
}

func (b *boolValue) IsBoolFlag() bool {
	return true
}

// getExecutionContext returns the directory context for execution
func getExecutionContext(path string) string {
	if path == "" {
		return "."
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "."
	}
	return filepath.Dir(absPath)
}

// parseArgValue parses a KEY=VALUE string into separate key and value
func parseArgValue(arg string) (string, string, error) {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid ARG format. Expected KEY=VALUE, got %s", arg)
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "\"'")
	return key, value, nil
}

// parseUserHost parses a user@host string
func parseUserHost(arg string) (string, string, bool) {
	if strings.Contains(arg, "@") {
		parts := strings.SplitN(arg, "@", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

// normalizeFlag removes leading dashes from flag names
func normalizeFlag(flag string) string {
	return strings.TrimLeft(flag, "-")
}

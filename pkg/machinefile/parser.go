package internal

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"
)

// initEnvVars initializes environment variables with built-in and predefined args
func initEnvVars(predefinedArgs map[string]string) map[string]string {
	envVars := make(map[string]string)
	
	// Add built-in ARGs
	envVars["BUILDKIT_SYNTAX"] = ""
	envVars["BUILD_DATE"] = fmt.Sprintf("%q", time.Now().UTC().Format("2006-01-02 15:04:05"))
	
	// Add predefined ARGs
	for k, v := range predefinedArgs {
		envVars[k] = v
		fmt.Printf("Using predefined ARG %s=%s\n", k, v)
	}
	
	return envVars
}

func ParseAndRunDockerfile(dockerfilePath string, runner Runner, predefinedArgs map[string]string) error {
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return fmt.Errorf("error opening Dockerfile: %w", err)
	}
	defer file.Close()

	// Initialize state
	envVars := initEnvVars(predefinedArgs)
	var currentUser string
	var command strings.Builder

	// Read file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle line continuation
		if command.Len() > 0 {
			if strings.HasSuffix(line, "\\") {
				command.WriteString(strings.TrimSuffix(line, "\\"))
				command.WriteString(" ")
				continue
			}
			command.WriteString(line)
			line = command.String()
			command.Reset()
		}

		// Parse instruction
		parts := strings.SplitN(line, " ", 2)
		instruction := parts[0]
		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}

		// Handle line continuation for new commands
		if strings.HasSuffix(args, "\\") {
			command.WriteString(strings.TrimPrefix(line, instruction+" "))
			command.WriteString(" ")
			continue
		}

		// Process instruction
		switch instruction {
		case "FROM":
			fmt.Printf("Ignoring command: %s\n", line)

		case "ARG":
			parts := strings.SplitN(args, "=", 2)
			key := parts[0]
			
			switch {
			case predefinedArgs[key] != "":
				envVars[key] = predefinedArgs[key]
				fmt.Printf("Using command line ARG %s=%s\n", key, envVars[key])
			case len(parts) == 2:
				value := strings.Trim(parts[1], "\"'")
				envVars[key] = expandVariables(value, envVars)
				fmt.Printf("Using Dockerfile default ARG %s=%s\n", key, envVars[key])
			default:
				envVars[key] = os.Getenv(key)
				if envVars[key] != "" {
					fmt.Printf("Using environment ARG %s=%s\n", key, envVars[key])
				} else {
					fmt.Printf("ARG %s has no value set\n", key)
				}
			}

		case "ENV":
			parts := strings.SplitN(args, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid ENV command: %s", args)
			}
			value := strings.Trim(parts[1], "\"'")
			envVars[parts[0]] = expandVariables(value, envVars)
			fmt.Printf("Set ENV %s=%s\n", parts[0], envVars[parts[0]])

		case "USER":
			currentUser = expandVariables(args, envVars)
			fmt.Printf("Switching to user: %s\n", currentUser)
			
			if _, ok := runner.(*LocalRunner); ok {
				if _, err := user.Lookup(currentUser); err != nil {
					return fmt.Errorf("error looking up user: %w", err)
				}
			}

		case "RUN":
			if err := runner.RunCommand(args, currentUser, envVars); err != nil {
				return fmt.Errorf("error running command: %w", err)
			}

		case "COPY", "ADD":
			parts := strings.Fields(args)
			if len(parts) != 2 {
				return fmt.Errorf("invalid %s command: requires exactly 2 arguments", instruction)
			}
			srcPattern := expandVariables(parts[0], envVars)
			dest := expandVariables(parts[1], envVars)
			if err := runner.CopyFile(srcPattern, dest, instruction == "ADD"); err != nil {
				return fmt.Errorf("error copying file: %w", err)
			}

		default:
			fmt.Printf("Unsupported command: %s\n", line)
		}
	}

	if command.Len() > 0 {
		return fmt.Errorf("command not properly terminated")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading Dockerfile: %w", err)
	}

	return nil
}
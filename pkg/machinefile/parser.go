package internal

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"
)

func ParseAndRunDockerfile(dockerfilePath string, runner Runner, predefinedArgs map[string]string) error {
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return fmt.Errorf("error opening Dockerfile: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentUser string
	envVars := make(map[string]string)
	
	// Add built-in ARGs
	currentTime := time.Now().UTC()
	envVars["BUILDKIT_SYNTAX"] = ""  // Common ARG in Containerfiles
	envVars["BUILD_DATE"] = `"` + currentTime.Format("2025-03-01 17:05:30") + `"`
	
	// Add predefined ARGs from command line
	for k, v := range predefinedArgs {
		envVars[k] = v
		fmt.Printf("Using predefined ARG %s=%s\n", k, v)
	}
	
	var runCommandBuilder strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "RUN ") || runCommandBuilder.Len() > 0 {
				if strings.HasSuffix(line, "\\") {
					runCommandBuilder.WriteString(strings.TrimSuffix(line, "\\"))
					runCommandBuilder.WriteString(" ")
				} else {
					if runCommandBuilder.Len() > 0 {
						runCommandBuilder.WriteString(line)
						command := runCommandBuilder.String()
						runCommandBuilder.Reset()

						if strings.HasPrefix(command, "RUN ") {
							command = strings.TrimPrefix(command, "RUN ")
						}

						if err := runner.RunCommand(command, currentUser, envVars); err != nil {
							return fmt.Errorf("error running command: %w", err)
						}
					} else {
						command := strings.TrimPrefix(line, "RUN ")
						if err := runner.RunCommand(command, currentUser, envVars); err != nil {
							return fmt.Errorf("error running command: %w", err)
						}
					}
				}
			} else if strings.HasPrefix(line, "COPY ") {
				parts := strings.Fields(strings.TrimPrefix(line, "COPY "))
				if len(parts) == 2 {
					srcPattern := expandVariables(parts[0], envVars)
					dest := expandVariables(parts[1], envVars)
					if err := runner.CopyFile(srcPattern, dest, false); err != nil {
						return fmt.Errorf("error copying file: %w", err)
					}
				} else {
					return fmt.Errorf("invalid COPY command: %s", line)
				}
			} else if strings.HasPrefix(line, "ADD ") {
				parts := strings.Fields(strings.TrimPrefix(line, "ADD "))
				if len(parts) == 2 {
					srcPattern := expandVariables(parts[0], envVars)
					dest := expandVariables(parts[1], envVars)
					if err := runner.CopyFile(srcPattern, dest, true); err != nil {
						return fmt.Errorf("error adding file: %w", err)
					}
				} else {
					return fmt.Errorf("invalid ADD command: %s", line)
				}
			} else if strings.HasPrefix(line, "USER ") {
				userValue := strings.TrimPrefix(line, "USER ")
				// Expand variables in USER command
				currentUser = expandVariables(userValue, envVars)
				fmt.Printf("Switching to user: %s\n", currentUser)
				
				if _, ok := runner.(*LocalRunner); ok {
					_, err := user.Lookup(currentUser)
					if err != nil {
						return fmt.Errorf("error looking up user: %w", err)
					}
				}
			} else if strings.HasPrefix(line, "ENV ") {
				env := strings.TrimPrefix(line, "ENV ")
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					// Expand variables in ENV values
					value := strings.Trim(parts[1], "\"'")
					envVars[parts[0]] = expandVariables(value, envVars)
					fmt.Printf("Set ENV %s=%s\n", parts[0], envVars[parts[0]])
				} else {
					return fmt.Errorf("invalid ENV command: %s", line)
				}
			} else if strings.HasPrefix(line, "ARG ") {
				arg := strings.TrimPrefix(line, "ARG ")
				parts := strings.SplitN(arg, "=", 2)
				key := parts[0]
                
				// First check if the ARG was provided via command line
				if value, exists := predefinedArgs[key]; exists {
					// Command line ARG takes precedence
					envVars[key] = value
					fmt.Printf("Using command line ARG %s=%s\n", key, value)
				} else if len(parts) == 2 {
					// If not provided via command line, use default from Dockerfile
					value := strings.Trim(parts[1], "\"'")
					envVars[key] = expandVariables(value, envVars)
					fmt.Printf("Using Dockerfile default ARG %s=%s\n", key, envVars[key])
				} else {
					// If no default value and not provided via command line, try environment
					envVars[key] = os.Getenv(key)
					if envVars[key] != "" {
						fmt.Printf("Using environment ARG %s=%s\n", key, envVars[key])
					} else {
						fmt.Printf("ARG %s has no value set\n", key)
					}
				}
			} else {
				fmt.Printf("Unsupported command: %s\n", line)
			}
		}
	}

	if runCommandBuilder.Len() > 0 {
		return fmt.Errorf("RUN command not properly terminated")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading Dockerfile: %w", err)
	}

	return nil
}
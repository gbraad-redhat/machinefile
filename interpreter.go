package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func runCommand(command string, userName string, envVars map[string]string) {
	var cmd *exec.Cmd
	
	if userName != "" {
		cmd = exec.Command("sudo", "-u", userName, "bash", "-c", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()
	for key, value := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	fmt.Printf("Executing command: %s\n", command)
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "Error running command: %s, Exit Code: %d\n", command, exitError.ExitCode())
		} else {
			fmt.Fprintf(os.Stderr, "Error running command: %s, %v\n", command, err)
		}
		os.Exit(1)
	}
}

func copyFile(srcPattern, dest, baseDir string) {
	srcPattern = filepath.Join(baseDir, srcPattern)
	srcPattern = filepath.Clean(srcPattern)
	dest = filepath.Clean(dest)

	matches, err := filepath.Glob(srcPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with glob pattern: %v\n", err)
		os.Exit(1)
	}

	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "No matches found for pattern: %s\n", srcPattern)
		os.Exit(1)
	}

	for _, src := range matches {
		srcInfo, err := os.Stat(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error stating source file: %v\n", err)
			os.Exit(1)
		}

		var copyErr error
		if srcInfo.IsDir() {
			copyErr = exec.Command("cp", "-r", src, dest).Run()
		} else {
			copyErr = exec.Command("cp", src, dest).Run()
		}

		if copyErr != nil {
			fmt.Fprintf(os.Stderr, "Error copying file: %v\n", copyErr)
			os.Exit(1)
		}
		fmt.Printf("Copied %s to %s\n", src, dest)
	}
}

func parseAndRunDockerfile(filepath, baseDir string) {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening Dockerfile: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentUser string
	envVars := make(map[string]string)
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
                        
                        // Extract the actual command part from the RUN directive if needed
                        if strings.HasPrefix(command, "RUN ") {
                            command = strings.TrimPrefix(command, "RUN ")
                        }
                        
						runCommand(command, currentUser, envVars)
					} else {
						command := strings.TrimPrefix(line, "RUN ")
						runCommand(command, currentUser, envVars)
					}
				}
			} else if strings.HasPrefix(line, "COPY ") {
				parts := strings.Fields(line)
				if len(parts) == 3 {
					srcPattern, dest := parts[1], parts[2]
					copyFile(srcPattern, dest, baseDir)
				} else {
					fmt.Fprintf(os.Stderr, "Invalid COPY command: %s\n", line)
					os.Exit(1)
				}
			} else if strings.HasPrefix(line, "USER ") {
				currentUser = strings.TrimPrefix(line, "USER ")
				fmt.Printf("Switching to user: %s\n", currentUser)

				// Check if the user exists
				_, err := user.Lookup(currentUser)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error looking up user: %v\n", err)
					os.Exit(1)
				}
			} else if strings.HasPrefix(line, "ENV ") {
				// Adding support for ENV command
				env := strings.TrimPrefix(line, "ENV ")
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					envVars[parts[0]] = parts[1]
					fmt.Printf("Set ENV %s=%s\n", parts[0], envVars[parts[0]])
				} else {
					fmt.Fprintf(os.Stderr, "Invalid ENV command: %s\n", line)
				}
			} else if strings.HasPrefix(line, "ARG ") {
				arg := strings.TrimPrefix(line, "ARG ")
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) == 2 {
					envVars[parts[0]] = parts[1]
				} else {
					envVars[parts[0]] = os.Getenv(parts[0])
				}
				fmt.Printf("Set ARG %s=%s\n", parts[0], envVars[parts[0]])
			} else {
				fmt.Printf("Unsupported command: %s\n", line)
			}
		}
	}

	if runCommandBuilder.Len() > 0 {
		fmt.Fprintf(os.Stderr, "Error: RUN command not properly terminated\n")
		os.Exit(1)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading Dockerfile: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s <Dockerfile path> <context>\n", os.Args[0])
		os.Exit(1)
	}

	dockerfilePath := os.Args[1]
	context := os.Args[2]

	parseAndRunDockerfile(dockerfilePath, context)
}

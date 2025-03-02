package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	machinefile "github.com/gbraad-redhat/machinefile/pkg/machinefile"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ", ")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func parseArgValue(arg string) (string, string, error) {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid ARG format. Expected KEY=VALUE, got %s", arg)
	}
	key := strings.TrimSpace(parts[0])
	// Handle quoted values
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "\"'")
	return key, value, nil
}

func getExecutionContext(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "."
	}
	return filepath.Dir(absPath)
}

func main() {
	var args arrayFlags
	sshHost := flag.String("host", "", "SSH host (if specified, executes remotely)")
	sshUser := flag.String("user", "", "SSH user (defaults to current user if running remotely)")
	sshKeyPath := flag.String("key", "", "Path to SSH private key (optional)")
	sshPassword := flag.String("password", "", "SSH password (optional)")
	askPassword := flag.Bool("ask-password", false, "Prompt for SSH password")

	// Add support for reading from stdin (for shebang use)
	readFromStdin := flag.Bool("stdin", false, "Read Dockerfile from standard input")

	// Add support for --arg flag
	flag.Var(&args, "arg", "Specify ARG values (format: --arg KEY=VALUE). Can be used multiple times")

	flag.Parse()

	// Get remaining arguments after flags
	remainingArgs := flag.Args()
	var dockerfilePath, context string
	var explicitContext bool

	if *readFromStdin {
		if len(remainingArgs) > 0 {
			// When executed via shebang, the script file will be the first argument
			// and set default context to the directory containing the Machinefile
			dockerfilePath = remainingArgs[0]
			context = getExecutionContext(dockerfilePath)

			// Process remaining arguments to find explicit context
			for i := 1; i < len(remainingArgs); i++ {
				arg := remainingArgs[i]
				if strings.HasPrefix(arg, "-") {

					// Handle flags
					if arg == "--arg" && i+1 < len(remainingArgs) {
						args = append(args, remainingArgs[i+1])
						i++ // Skip the next argument as it's part of --arg
					} else if strings.HasPrefix(arg, "--arg=") {
						value := strings.TrimPrefix(arg, "--arg=")
						args = append(args, value)
					}
				} else if !explicitContext {
					// First non-flag argument after the Machinefile path is the context
					context = arg
					explicitContext = true
				}
			}
		} else {
			// Traditional stdin reading
			tempFile, err := os.CreateTemp("", "machinefile-*.tmp")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating temporary file: %v\n", err)
				os.Exit(1)
			}
			defer os.Remove(tempFile.Name())

			writer := bufio.NewWriter(tempFile)
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				_, err := writer.WriteString(scanner.Text() + "\n")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to temporary file: %v\n", err)
					os.Exit(1)
				}
			}
			writer.Flush()
			tempFile.Close()

			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "Error reading from standard input: %v\n", err)
				os.Exit(1)
			}

			dockerfilePath = tempFile.Name()
			context = "."
		}
	} else if len(remainingArgs) == 2 {
		dockerfilePath = remainingArgs[0]
		context = remainingArgs[1]
	} else if len(remainingArgs) == 1 {
		dockerfilePath = remainingArgs[0]
		context = getExecutionContext(dockerfilePath)
	} else {
		fmt.Printf("Usage: %s [options] <Dockerfile path> [context]\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parse ARG values
	predefinedArgs := make(map[string]string)
	for _, arg := range args {
		key, value, err := parseArgValue(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing ARG: %v\n", err)
			os.Exit(1)
		}
		predefinedArgs[key] = value
	}

	var runner machinefile.Runner

	if *sshHost != "" {
		sshUserName := *sshUser
		if sshUserName == "" {
			currentUser, err := user.Current()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting current user: %v\n", err)
				os.Exit(1)
			}
			sshUserName = currentUser.Username
		}

		runner = &machinefile.SSHRunner{
			BaseDir:     context,
			SshHost:     *sshHost,
			SshUser:     sshUserName,
			SshKeyPath:  *sshKeyPath,
			SshPassword: *sshPassword,
			AskPassword: *askPassword,
		}

		fmt.Printf("Running on remote host %s as user %s\n", *sshHost, sshUserName)
	} else {
		runner = &machinefile.LocalRunner{
			BaseDir: context,
		}
		fmt.Printf("Running locally in context: %s\n", context)
	}

	/* err := */machinefile.ParseAndRunDockerfile(dockerfilePath, runner, predefinedArgs)
	/*
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running Dockerfile: %v\n", err)
		os.Exit(1)
	}
	*/
}
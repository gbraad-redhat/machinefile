package main

import (
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
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, "\"'")
	return key, value, nil
}

func parseUserHost(arg string) (string, string, bool) {
	if strings.Contains(arg, "@") {
		parts := strings.SplitN(arg, "@", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

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

func main() {
	var args arrayFlags
	
	// Define all flags upfront
	sshHost := flag.String("host", "", "SSH host (if specified, executes remotely)")
	sshUser := flag.String("user", "", "SSH user (defaults to current user if running remotely)")
	sshKeyPath := flag.String("key", "", "Path to SSH private key (optional)")
	sshPassword := flag.String("password", "", "SSH password (optional)")
	askPassword := flag.Bool("ask-password", false, "Prompt for SSH password")
	stdinMode := flag.Bool("stdin", false, "Read Dockerfile from stdin (used with shebang)")
	
	// Container-related flags
	container := flag.String("container", "", "Podman container name")
	connection := flag.String("connection", "", "Podman connection name")
	podmanBinary := flag.String("podman-binary", "podman", "Path to Podman binary")

	flag.Var(&args, "arg", "Specify ARG values (format: --arg KEY=VALUE). Can be used multiple times")

	// Parse flags
	flag.Parse()

	var dockerfilePath string
	var context string
	predefinedArgs := make(map[string]string)
	predefinedArgs["MACHINEFILE"] = "0.7.0"

	remainingArgs := flag.Args()
	if *stdinMode {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: insufficient arguments for shebang mode\n")
			os.Exit(1)
		}
		dockerfilePath = os.Args[2]
		context = getExecutionContext(dockerfilePath)

		// Process remaining arguments
		for i := 3; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch arg {
			case "-container", "--container":
				if i+1 < len(os.Args) {
					*container = os.Args[i+1]
					i++
				}
			case "-connection", "--connection":
				if i+1 < len(os.Args) {
					*connection = os.Args[i+1]
					i++
				}
			case "-podman-binary", "--podman-binary":
				if i+1 < len(os.Args) {
					*podmanBinary = os.Args[i+1]
					i++
				}
			case "--arg":
				if i+1 < len(os.Args) {
					key, value, err := parseArgValue(os.Args[i+1])
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error parsing ARG: %v\n", err)
						os.Exit(1)
					}
					predefinedArgs[key] = value
					i++
				}
			default:
				if user, host, ok := parseUserHost(arg); ok {
					*sshUser = user
					*sshHost = host
				}
			}
		}
	} else {
		// Handle normal mode
		var processedArgs []string

		// First check for user@host pattern
		if len(remainingArgs) > 0 {
			if user, host, ok := parseUserHost(remainingArgs[0]); ok {
				*sshUser = user
				*sshHost = host
				remainingArgs = remainingArgs[1:]
			}
		}

		// Process remaining arguments but keep track of real arguments
		for i := 0; i < len(remainingArgs); i++ {
			arg := remainingArgs[i]
			if arg == "--arg" && i+1 < len(remainingArgs) {
				key, value, err := parseArgValue(remainingArgs[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing ARG: %v\n", err)
					os.Exit(1)
				}
				predefinedArgs[key] = value
				i++
				continue
			}
			processedArgs = append(processedArgs, arg)
		}

		// Now process the real arguments
		if len(processedArgs) >= 2 {
			dockerfilePath = processedArgs[0]
			context = processedArgs[1]
		} else if len(processedArgs) == 1 {
			dockerfilePath = processedArgs[0]
			context = getExecutionContext(dockerfilePath)
		} else {
			fmt.Printf("Usage: %s [user@host] [options] <Dockerfile path> [context]\n", os.Args[0])
			fmt.Println("Options:")
			flag.PrintDefaults()
			os.Exit(1)
		}

		// Process ARG values from flags
		for _, arg := range args {
			key, value, err := parseArgValue(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing ARG: %v\n", err)
				os.Exit(1)
			}
			predefinedArgs[key] = value
		}
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
	} else if *container != "" {
		runner = &machinefile.PodmanRunner{
			BaseDir:        context,
			ContainerName:  *container,
			ConnectionName: *connection,
			PodmanBinary:  *podmanBinary,
		}

		fmt.Printf("Running in Podman container %s\n", *container)
		if *connection != "" {
			fmt.Printf("Using Podman connection: %s\n", *connection)
		}
	} else {
		runner = &machinefile.LocalRunner{
			BaseDir: context,
		}
		fmt.Printf("Running locally in context: %s\n", context)
	}

	err := machinefile.ParseAndRunDockerfile(dockerfilePath, runner, predefinedArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running Dockerfile: %v\n", err)
		os.Exit(1)
	}
}
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

// stringValue is a helper type to satisfy flag.Value interface
type stringValue string

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) String() string {
	return string(*s)
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

func main() {
	// Runner type flags
	useLocal := flag.Bool("local", false, "Force local runner")
	usePodman := flag.Bool("podman", false, "Force Podman runner")
	useSSH := flag.Bool("ssh", false, "Force SSH runner")

	// File and context flags with shorthands
	dockerFile := new(string)
	contextPath := new(string)
	fFlag := newFlagWithShorthand("file", "f", (*stringValue)(dockerFile), "Path to the Containerfile/Dockerfile to execute")
	cFlag := newFlagWithShorthand("context", "c", (*stringValue)(contextPath), "Context path for execution")

	// Register both long and short forms
	flag.Var(fFlag.value, fFlag.name, fFlag.usage)
	flag.Var(fFlag.value, fFlag.shorthand, fFlag.usage)
	flag.Var(cFlag.value, cFlag.name, cFlag.usage)
	flag.Var(cFlag.value, cFlag.shorthand, cFlag.usage)

	// SSH-related flags with shorthands
	sshHostValue := new(string)
	sshUserValue := new(string)
	hFlag := newFlagWithShorthand("host", "H", (*stringValue)(sshHostValue), "SSH host for remote execution")
	uFlag := newFlagWithShorthand("user", "u", (*stringValue)(sshUserValue), "SSH user for remote execution")

	flag.Var(hFlag.value, hFlag.name, hFlag.usage)
	flag.Var(hFlag.value, hFlag.shorthand, hFlag.usage)
	flag.Var(uFlag.value, uFlag.name, uFlag.usage)
	flag.Var(uFlag.value, uFlag.shorthand, uFlag.usage)

	// Other SSH flags
	sshKeyPath := flag.String("key", "", "Path to SSH private key (optional)")
	sshPassword := flag.String("password", "", "SSH password (optional)")
	askPassword := flag.Bool("ask-password", false, "Prompt for SSH password")
	stdinMode := flag.Bool("stdin", false, "Read Dockerfile from stdin (used with shebang)")

	// Container-related flags
	container := flag.String("container", "", "Podman container name")
	connection := flag.String("connection", "", "Podman connection name")
	podmanBinary := flag.String("podman-binary", "podman", "Path to Podman binary")

	// ARG values
	var args []string
	flag.Func("arg", "Specify ARG values (format: -arg or --arg KEY=VALUE)", func(value string) error {
		args = append(args, value)
		return nil
	})

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] [CONTAINERFILE] [CONTEXT]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")

		// Get the default flag set
		flag.CommandLine.VisitAll(func(f *flag.Flag) {
			// Skip shorthand flags in the main listing
			if len(f.Name) == 1 {
				for _, fs := range flagsWithShorthands {
					if fs.shorthand == f.Name {
						return
					}
				}
			}

			// For flags with shorthands, show both forms
			var name string
			for _, fs := range flagsWithShorthands {
				if fs.name == f.Name {
					name = fmt.Sprintf("-%s, --%s", fs.shorthand, fs.name)
					fmt.Fprintf(os.Stderr, "  %-20s %s\n", name, fs.usage)
					return
				}
			}

			// Regular flags
			if name == "" {
				name = fmt.Sprintf("--%s", f.Name)
				fmt.Fprintf(os.Stderr, "  %-20s %s\n", name, f.Usage)
			}
		})

		fmt.Fprintf(os.Stderr, "\nPositional Arguments:\n")
		fmt.Fprintf(os.Stderr, "  CONTAINERFILE  Path to the Containerfile/Dockerfile (can also be specified with -f, --file)\n")
		fmt.Fprintf(os.Stderr, "  CONTEXT       Context path for execution (can also be specified with -c, --context)\n")
	}

	// Parse flags
	flag.Parse()

	// Validate runner flags
	runnerFlags := 0
	if *useLocal {
		runnerFlags++
	}
	if *usePodman {
		runnerFlags++
	}
	if *useSSH {
		runnerFlags++
	}
	if runnerFlags > 1 {
		fmt.Fprintf(os.Stderr, "Error: Only one runner flag (--local, --ssh, --podman) can be specified\n")
		os.Exit(1)
	}

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
			normalizedArg := strings.TrimLeft(arg, "-")

			switch normalizedArg {
			case "local":
				*useLocal = true
			case "podman":
				*usePodman = true
			case "ssh":
				*useSSH = true
			case "f", "file":
				if i+1 < len(os.Args) {
					dockerfilePath = os.Args[i+1]
					i++
				}
			case "c", "context":
				if i+1 < len(os.Args) {
					context = os.Args[i+1]
					i++
				}
			case "container":
				if i+1 < len(os.Args) {
					*container = os.Args[i+1]
					i++
				}
			case "connection":
				if i+1 < len(os.Args) {
					*connection = os.Args[i+1]
					i++
				}
			case "podman-binary":
				if i+1 < len(os.Args) {
					*podmanBinary = os.Args[i+1]
					i++
				}
			case "arg":
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
					*sshUserValue = user
					*sshHostValue = host
				}
			}
		}
	} else {
		// Handle file and context from flags first
		if *dockerFile != "" {
			dockerfilePath = string(*dockerFile)
		}
		if *contextPath != "" {
			context = string(*contextPath)
		}

		// Handle positional arguments if flags are not set
		switch len(remainingArgs) {
		case 2:
			if dockerfilePath == "" {
				dockerfilePath = remainingArgs[0]
			}
			if context == "" {
				context = remainingArgs[1]
			}
		case 1:
			if dockerfilePath == "" {
				dockerfilePath = remainingArgs[0]
			}
			if context == "" {
				context = getExecutionContext(dockerfilePath)
			}
		case 0:
			if dockerfilePath == "" {
				fmt.Fprintf(os.Stderr, "Error: No Containerfile specified. Use -f/--file or provide as argument\n")
				flag.Usage()
				os.Exit(1)
			}
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

	// Set context if still empty
	if context == "" {
		context = getExecutionContext(dockerfilePath)
	}

	var runner machinefile.Runner

	// Determine which runner to use based on flags and parameters
	switch {
	case *useSSH || (!*useLocal && !*usePodman && *sshHostValue != ""):
		sshUsername := string(*sshUserValue)
		if sshUsername == "" {
			currentUser, err := user.Current()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting current user: %v\n", err)
				os.Exit(1)
			}
			sshUsername = currentUser.Username
		}

		if *sshHostValue == "" {
			fmt.Fprintf(os.Stderr, "Error: SSH runner requires -H/--host parameter\n")
			os.Exit(1)
		}

		runner = &machinefile.SSHRunner{
			BaseDir:     context,
			SshHost:     string(*sshHostValue),
			SshUser:     sshUsername,
			SshKeyPath:  *sshKeyPath,
			SshPassword: *sshPassword,
			AskPassword: *askPassword,
		}

		fmt.Printf("Running on remote host %s as user %s\n", string(*sshHostValue), sshUsername)

	case *usePodman || (!*useLocal && !*useSSH && *container != ""):
		if *container == "" {
			fmt.Fprintf(os.Stderr, "Error: Podman runner requires --container parameter\n")
			os.Exit(1)
		}

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

	case *useLocal:
		runner = &machinefile.LocalRunner{
			BaseDir: context,
		}
		fmt.Printf("Running locally in context: %s\n", context)

	default:
		// Default to local runner if no specific runner is selected
		runner = &machinefile.LocalRunner{
			BaseDir: context,
		}
		fmt.Printf("Running locally in context: %s (default)\n", context)
	}

	err := machinefile.ParseAndRunDockerfile(dockerfilePath, runner, predefinedArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running Dockerfile: %v\n", err)
		os.Exit(1)
	}
}
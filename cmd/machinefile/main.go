package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	machinefile "github.com/gbraad-redhat/machinefile/pkg/machinefile"
)

const (
	VERSION     = "0.8.7"
	DATE_FORMAT = "2006-01-02 15:04:05"
)

func main() {
	// Help flag
	helpRequested := new(bool)
	helpFlag := newFlagWithShorthand("help", "h", newBoolValue(helpRequested), "Show usage message")
	flag.Var(helpFlag.value, helpFlag.name, helpFlag.usage)
	flag.Var(helpFlag.value, helpFlag.shorthand, helpFlag.usage)

	// Runner type flags with shorthands
	useLocalValue := new(bool)
	usePodmanValue := new(bool)
	useSSHValue := new(bool)

	lFlag := newFlagWithShorthand("local", "l", newBoolValue(useLocalValue), "Select local runner")
	pFlag := newFlagWithShorthand("podman", "p", newBoolValue(usePodmanValue), "Select Podman runner")
	sFlag := newFlagWithShorthand("ssh", "s", newBoolValue(useSSHValue), "Select SSH runner")

	flag.Var(lFlag.value, lFlag.name, lFlag.usage)
	flag.Var(lFlag.value, lFlag.shorthand, lFlag.usage)
	flag.Var(pFlag.value, pFlag.name, pFlag.usage)
	flag.Var(pFlag.value, pFlag.shorthand, pFlag.usage)
	flag.Var(sFlag.value, sFlag.name, sFlag.usage)
	flag.Var(sFlag.value, sFlag.shorthand, sFlag.usage)

	// File and context flags with shorthands
	dockerFile := new(string)
	contextPath := new(string)
	fileFlag := newFlagWithShorthand("file", "f", (*stringValue)(dockerFile), "Path to the Containerfile/Dockerfile to execute")
	contextFlag := newFlagWithShorthand("context", "c", (*stringValue)(contextPath), "Context path for execution")

	// Continue on error
	continueOnError := flag.Bool("continue-on-error", false, "Continue execution on error")

	// Register both long and short forms
	flag.Var(fileFlag.value, fileFlag.name, fileFlag.usage)
	flag.Var(fileFlag.value, fileFlag.shorthand, fileFlag.usage)
	flag.Var(contextFlag.value, contextFlag.name, contextFlag.usage)
	flag.Var(contextFlag.value, contextFlag.shorthand, contextFlag.usage)

	// SSH-related flags with shorthands
	sshHostValue := new(string)
	sshUserValue := new(string)
	hostFlag := newFlagWithShorthand("host", "H", (*stringValue)(sshHostValue), "SSH host for remote execution")
	userFlag := newFlagWithShorthand("user", "u", (*stringValue)(sshUserValue), "SSH user for remote execution")

	flag.Var(hostFlag.value, hostFlag.name, hostFlag.usage)
	flag.Var(hostFlag.value, hostFlag.shorthand, hostFlag.usage)
	flag.Var(userFlag.value, userFlag.name, userFlag.usage)
	flag.Var(userFlag.value, userFlag.shorthand, userFlag.usage)

	// Other SSH flags
	sshKeyPath := flag.String("key", "", "Path to SSH private key (optional)")
	sshPassword := flag.String("password", "", "SSH password (optional)")
	askPassword := flag.Bool("ask-password", false, "Prompt for SSH password")
	stdinMode := flag.Bool("stdin", false, "Read Dockerfile from stdin (used with shebang)")

	// Container-related flags
	containerName := new(string)
	nameFlag := newFlagWithShorthand("name", "n", (*stringValue)(containerName), "Podman container name")

	flag.Var(nameFlag.value, nameFlag.name, nameFlag.usage)
	flag.Var(nameFlag.value, nameFlag.shorthand, nameFlag.usage)

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
		fmt.Fprintf(os.Stderr, "\nMachinefile version: %s\n", VERSION)

		// Print each category
		for _, category := range categories {
			fmt.Fprintf(os.Stderr, "\n%s:\n", category.name)

			flag.VisitAll(func(f *flag.Flag) {
				if flagInCategory(f.Name, category.flags) {
					help := printFlagHelp(f)
					if help != "" {
						fmt.Fprintln(os.Stderr, help)
					}
				}
			})
		}

		fmt.Fprintf(os.Stderr, "\nPositional Arguments:\n")
		fmt.Fprintf(os.Stderr, "  CONTAINERFILE  Path to the Containerfile/Dockerfile (can also be specified with -f, --file)\n")
		fmt.Fprintf(os.Stderr, "  CONTEXT        Context path for execution (can also be specified with -c, --context)\n")
	}

	// Parse flags
	flag.Parse()

	// Check for help flag early, before any other processing including stdin mode
	if bool(*helpRequested) {
		flag.Usage()
		os.Exit(0)
	}

	var dockerfilePath string
	var context string
	predefinedArgs := make(map[string]string)
	predefinedArgs["MACHINEFILE"] = VERSION
	predefinedArgs["BUILDKIT_SYNTAX"] = "" // Common ARG in Containerfiles
	predefinedArgs["BUILD_DATE"] = `"` + time.Now().UTC().Format(DATE_FORMAT) + `"`

	remainingArgs := flag.Args()
	if *stdinMode {
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: insufficient arguments for shebang mode\n")
			os.Exit(1)
		}

		// Process all remaining arguments to check for help flag in stdin mode
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "-h" || arg == "--help" {
				flag.Usage()
				os.Exit(0)
			}
		}

		dockerfilePath = os.Args[2]
		context = getExecutionContext(dockerfilePath)

		// Process remaining arguments
		for i := 3; i < len(os.Args); i++ {
			arg := os.Args[i]
			normalizedArg := normalizeFlag(arg)

			switch normalizedArg {
			case "continue-on-error":
				*continueOnError = true
			case "l", "local":
				*useLocalValue = true
			case "p", "podman":
				*usePodmanValue = true
			case "s", "ssh":
				*useSSHValue = true
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
			case "n", "name":
				if i+1 < len(os.Args) {
					*containerName = os.Args[i+1]
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
		// Fix to parse user@host from positional arguments
		for i := 0; i < len(remainingArgs); i++ {
			arg := remainingArgs[i]
			if strings.HasPrefix(arg, "-") {
				continue
			}

			if user, host, ok := parseUserHost(arg); ok {
				*sshUserValue = user
				*sshHostValue = host
				continue
			}

			if dockerfilePath == "" {
				dockerfilePath = arg
			} else if context == "" {
				context = arg
			}
		}

		// Handle file and context from flags first
		if *dockerFile != "" {
			dockerfilePath = string(*dockerFile)
		}
		if *contextPath != "" {
			context = string(*contextPath)
		}

		// If no context specified, use Containerfile's directory
		if context == "" {
			context = getExecutionContext(dockerfilePath)
		}

		// Process ARG values
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
	case bool(*useSSHValue) || (!bool(*useLocalValue) && !bool(*usePodmanValue) && *sshHostValue != ""):
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

	case bool(*usePodmanValue) || (!bool(*useLocalValue) && !bool(*useSSHValue) && *containerName != ""):
		if *containerName == "" {
			fmt.Fprintf(os.Stderr, "Error: Podman runner requires -n/--name parameter\n")
			os.Exit(1)
		}

		runner = &machinefile.PodmanRunner{
			BaseDir:        context,
			ContainerName:  string(*containerName),
			ConnectionName: *connection,
			PodmanBinary:   *podmanBinary,
		}

		fmt.Printf("Running in Podman container %s\n", string(*containerName))
		if *connection != "" {
			fmt.Printf("Using Podman connection: %s\n", *connection)
		}

	case bool(*useLocalValue):
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

	err := machinefile.ParseAndRunDockerfile(dockerfilePath, runner, predefinedArgs, *continueOnError)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running Dockerfile: %v\n", err)
		if !*continueOnError {
			os.Exit(1)
		}
	}
}

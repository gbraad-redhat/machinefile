package main

import (
	"fmt"
	"os"
	"flag"
	"os/user"
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

func main() {
	var args arrayFlags
	sshHost := flag.String("host", "", "SSH host (if specified, executes remotely)")
	sshUser := flag.String("user", "", "SSH user (defaults to current user if running remotely)")
	sshKeyPath := flag.String("key", "", "Path to SSH private key (optional)")
	sshPassword := flag.String("password", "", "SSH password (optional)")
	askPassword := flag.Bool("ask-password", false, "Prompt for SSH password")
	
	// Add support for --arg flag
	flag.Var(&args, "arg", "Specify ARG values (format: --arg KEY=VALUE). Can be used multiple times")
	
	flag.Parse()
	
	// Get remaining arguments after flags
	remainingArgs := flag.Args()
	if len(remainingArgs) != 2 {
		fmt.Printf("Usage: %s [options] <Dockerfile path> <context>\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}
	
	dockerfilePath := remainingArgs[0]
	context := remainingArgs[1]
	
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
		fmt.Printf("Running locally\n")
	}
	
	machinefile.ParseAndRunDockerfile(dockerfilePath, runner, predefinedArgs)
}

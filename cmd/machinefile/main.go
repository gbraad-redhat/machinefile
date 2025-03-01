package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"golang.org/x/crypto/ssh/terminal"
)

type Runner interface {
	RunCommand(command string, userName string, envVars map[string]string) error
	CopyFile(srcPattern, dest string, isAdd bool) error
}

type LocalRunner struct {
	baseDir string
}

type SSHRunner struct {
	baseDir     string
	sshHost     string
	sshUser     string
	sshKeyPath  string
	sshPassword string
	askPassword bool
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ", ")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (lr *LocalRunner) RunCommand(command string, userName string, envVars map[string]string) error {
	expandedCommand := expandVariables(command, envVars)

	var cmd *exec.Cmd

	if userName != "" {
		cmd = exec.Command("sudo", "-u", userName, "bash", "-c", expandedCommand)
	} else {
		cmd = exec.Command("bash", "-c", expandedCommand)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()
	for key, value := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	fmt.Printf("Executing command: %s\n", expandedCommand)
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "Error running command: %s, Exit Code: %d\n", expandedCommand, exitError.ExitCode())
		} else {
			fmt.Fprintf(os.Stderr, "Error running command: %s, %v\n", expandedCommand, err)
		}
		return err
	}
	return nil
}

func (lr *LocalRunner) CopyFile(srcPattern, dest string, isAdd bool) error {
	srcPattern = filepath.Join(lr.baseDir, srcPattern)
	srcPattern = filepath.Clean(srcPattern)
	dest = filepath.Clean(dest)

	matches, err := filepath.Glob(srcPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with glob pattern: %v\n", err)
		return err
	}

	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "No matches found for pattern: %s\n", srcPattern)
		return fmt.Errorf("no matches found")
	}

	for _, src := range matches {
		srcInfo, err := os.Stat(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error stating source file: %v\n", err)
			return err
		}

		var copyErr error
		if srcInfo.IsDir() {
			if isAdd {
				os.MkdirAll(dest, 0755)
				// Use cp -a to preserve permissions, ownership, timestamps, etc.
				copyErr = exec.Command("bash", "-c", fmt.Sprintf("cp -a %s/* %s/", src, dest)).Run()
			} else {
				// Use cp -a to preserve permissions, ownership, timestamps, etc.
				copyErr = exec.Command("cp", "-a", src, dest).Run()
			}
		} else {
			// Use cp -p to preserve permissions, ownership, timestamps
			copyErr = exec.Command("cp", "-p", src, dest).Run()
		}

		if copyErr != nil {
			fmt.Fprintf(os.Stderr, "Error copying file: %v\n", copyErr)
			return copyErr
		}

		if isAdd {
			fmt.Printf("Added contents of %s to %s\n", src, dest)
		} else {
			fmt.Printf("Copied %s to %s\n", src, dest)
		}
	}
	return nil
}

func getSSHAuth(runner *SSHRunner) []string {
	var sshArgs []string
	
	if runner.askPassword {
		fmt.Printf("Enter SSH password for %s@%s: ", runner.sshUser, runner.sshHost)
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		runner.sshPassword = string(bytePassword)
	}
	
	if runner.sshPassword != "" {
		if _, err := exec.LookPath("sshpass"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: sshpass is not installed. Please install it to use password authentication.\n")
			os.Exit(1)
		}
		sshArgs = append(sshArgs, "sshpass", "-p", runner.sshPassword, "ssh")
	} else {
		sshArgs = append(sshArgs, "ssh")
	}
	
	if runner.sshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", runner.sshKeyPath)
	}
	
	sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=no")
	
	return sshArgs
}

func (sr *SSHRunner) RunCommand(command string, userName string, envVars map[string]string) error {
	expandedCommand := expandVariables(command, envVars)
	sshCommand := expandedCommand
	
	if len(envVars) > 0 {
		envPrefix := ""
		for key, value := range envVars {
			envPrefix += fmt.Sprintf("%s=%s ", key, value)
		}
		sshCommand = envPrefix + sshCommand
	}
	
	if userName != "" {
		sshCommand = fmt.Sprintf("sudo -u %s bash -c '%s'", userName, strings.Replace(sshCommand, "'", "'\"'\"'", -1))
	}
	
	sshArgs := getSSHAuth(sr)
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", sr.sshUser, sr.sshHost), sshCommand)
	
	cmd := exec.Command(sshArgs[0], sshArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	fmt.Printf("Executing remote command: %s\n", sshCommand)
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running remote command: %s, %v\n", command, err)
		return err
	}
	return nil
}

func (sr *SSHRunner) CopyFile(srcPattern, dest string, isAdd bool) error {
    srcPattern = filepath.Join(sr.baseDir, srcPattern)
    srcPattern = filepath.Clean(srcPattern)
    dest = filepath.Clean(dest)
    
    matches, err := filepath.Glob(srcPattern)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error with glob pattern: %v\n", err)
        return err
    }
    
    if len(matches) == 0 {
        fmt.Fprintf(os.Stderr, "No matches found for pattern: %s\n", srcPattern)
        return fmt.Errorf("no matches found")
    }
    
    for _, src := range matches {
        srcInfo, err := os.Stat(src)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error stating source file: %v\n", err)
            return err
        }
        
        remoteTmpDir := fmt.Sprintf("/tmp/dockerfile-run-%d", time.Now().UnixNano())
        err = sr.RunCommand(fmt.Sprintf("mkdir -p %s", remoteTmpDir), "", nil)
        if err != nil {
            return err
        }
        
        // Fix for scp command construction
        var scpArgs []string
        
        if sr.sshPassword != "" {
            if _, err := exec.LookPath("sshpass"); err != nil {
                fmt.Fprintf(os.Stderr, "Error: sshpass is not installed. Please install it to use password authentication.\n")
                os.Exit(1)
            }
            scpArgs = append(scpArgs, "sshpass", "-p", sr.sshPassword, "scp")
        } else {
            scpArgs = append(scpArgs, "scp")
        }
        
        // Add common options
        scpArgs = append(scpArgs, "-o", "StrictHostKeyChecking=no")
        
        // Add key if specified
        if sr.sshKeyPath != "" {
            scpArgs = append(scpArgs, "-i", sr.sshKeyPath)
        }
        
        // Add -p flag to preserve file attributes
        scpArgs = append(scpArgs, "-p", "-r")
        
        // Add source and destination
        scpArgs = append(scpArgs, src, fmt.Sprintf("%s@%s:%s/", sr.sshUser, sr.sshHost, remoteTmpDir))
        
        scpCmd := exec.Command(scpArgs[0], scpArgs[1:]...)
        scpCmd.Stdout = os.Stdout
        scpCmd.Stderr = os.Stderr
        
        if err := scpCmd.Run(); err != nil {
            fmt.Fprintf(os.Stderr, "Error copying file to remote host: %v\n", err)
            return err
        }
        
        srcBase := filepath.Base(src)
        remoteSrc := filepath.Join(remoteTmpDir, srcBase)
        
        var mvCommand string
        if srcInfo.IsDir() && isAdd {
            mvCommand = fmt.Sprintf("mkdir -p %s && cp -a %s/* %s/ && rm -rf %s", dest, remoteSrc, dest, remoteTmpDir)
        } else {
            mvCommand = fmt.Sprintf("mkdir -p $(dirname %s) && cp -a %s %s && rm -rf %s", dest, remoteSrc, dest, remoteTmpDir)
        }
        
        if err := sr.RunCommand(mvCommand, "", nil); err != nil {
            return err
        }
        
        if isAdd {
            fmt.Printf("Added contents of %s to %s on %s (preserving attributes)\n", src, dest, sr.sshHost)
        } else {
            fmt.Printf("Copied %s to %s on %s (preserving attributes)\n", src, dest, sr.sshHost)
        }
    }
    
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

func parseAndRunDockerfile(dockerfilePath string, runner Runner, predefinedArgs map[string]string) {
	file, err := os.Open(dockerfilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening Dockerfile: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentUser string
	envVars := make(map[string]string)
	
	// Add built-in ARGs
	currentTime := time.Now().UTC()
	envVars["BUILDKIT_SYNTAX"] = ""  // Common ARG in Containerfiles
	envVars["BUILD_DATE"] = currentTime.Format("2025-03-01 17:05:30")
	
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
							os.Exit(1)
						}
					} else {
						command := strings.TrimPrefix(line, "RUN ")
						if err := runner.RunCommand(command, currentUser, envVars); err != nil {
							os.Exit(1)
						}
					}
				}
			} else if strings.HasPrefix(line, "COPY ") {
				parts := strings.Fields(strings.TrimPrefix(line, "COPY "))
				if len(parts) == 2 {
					srcPattern := expandVariables(parts[0], envVars)
					dest := expandVariables(parts[1], envVars)
					if err := runner.CopyFile(srcPattern, dest, false); err != nil {
						os.Exit(1)
					}
				} else {
					fmt.Fprintf(os.Stderr, "Invalid COPY command: %s\n", line)
					os.Exit(1)
				}
			} else if strings.HasPrefix(line, "ADD ") {
				parts := strings.Fields(strings.TrimPrefix(line, "ADD "))
				if len(parts) == 2 {
					srcPattern := expandVariables(parts[0], envVars)
					dest := expandVariables(parts[1], envVars)
					if err := runner.CopyFile(srcPattern, dest, true); err != nil {
						os.Exit(1)
					}
				} else {
					fmt.Fprintf(os.Stderr, "Invalid ADD command: %s\n", line)
					os.Exit(1)
				}
			} else if strings.HasPrefix(line, "USER ") {
				userValue := strings.TrimPrefix(line, "USER ")
				// Expand variables in USER command
				currentUser = expandVariables(userValue, envVars)
				fmt.Printf("Switching to user: %s\n", currentUser)
				
				if _, ok := runner.(*LocalRunner); ok {
					_, err := user.Lookup(currentUser)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error looking up user: %v\n", err)
						os.Exit(1)
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
					fmt.Fprintf(os.Stderr, "Invalid ENV command: %s\n", line)
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
		fmt.Fprintf(os.Stderr, "Error: RUN command not properly terminated\n")
		os.Exit(1)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading Dockerfile: %v\n", err)
		os.Exit(1)
	}
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
	
	var runner Runner
	
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
		
		runner = &SSHRunner{
			baseDir:     context,
			sshHost:     *sshHost,
			sshUser:     sshUserName,
			sshKeyPath:  *sshKeyPath,
			sshPassword: *sshPassword,
			askPassword: *askPassword,
		}
		
		fmt.Printf("Running on remote host %s as user %s\n", *sshHost, sshUserName)
	} else {
		runner = &LocalRunner{
			baseDir: context,
		}
		fmt.Printf("Running locally\n")
	}
	
	parseAndRunDockerfile(dockerfilePath, runner, predefinedArgs)
}

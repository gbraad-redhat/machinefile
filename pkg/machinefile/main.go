package internal

import (
	"bufio"
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
	BaseDir string
}

type SSHRunner struct {
	BaseDir     string
	SshHost     string
	SshUser     string
	SshKeyPath  string
	SshPassword string
	AskPassword bool
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
	srcPattern = filepath.Join(lr.BaseDir, srcPattern)
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

func getSSHAuth(sr *SSHRunner) []string {
	var sshArgs []string
	
	if sr.AskPassword {
		fmt.Printf("Enter SSH password for %s@%s: ", sr.SshUser, sr.SshHost)
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		sr.SshPassword = string(bytePassword)
	}
	
	if sr.SshPassword != "" {
		if _, err := exec.LookPath("sshpass"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: sshpass is not installed. Please install it to use password authentication.\n")
			os.Exit(1)
		}
		sshArgs = append(sshArgs, "sshpass", "-p", sr.SshPassword, "ssh")
	} else {
		sshArgs = append(sshArgs, "ssh")
	}
	
	if sr.SshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", sr.SshKeyPath)
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
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", sr.SshUser, sr.SshHost), sshCommand)
	
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
    srcPattern = filepath.Join(sr.BaseDir, srcPattern)
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
        
        if sr.SshPassword != "" {
            if _, err := exec.LookPath("sshpass"); err != nil {
                fmt.Fprintf(os.Stderr, "Error: sshpass is not installed. Please install it to use password authentication.\n")
                os.Exit(1)
            }
            scpArgs = append(scpArgs, "sshpass", "-p", sr.SshPassword, "scp")
        } else {
            scpArgs = append(scpArgs, "scp")
        }
        
        // Add common options
        scpArgs = append(scpArgs, "-o", "StrictHostKeyChecking=no")
        
        // Add key if specified
        if sr.SshKeyPath != "" {
            scpArgs = append(scpArgs, "-i", sr.SshKeyPath)
        }
        
        // Add -p flag to preserve file attributes
        scpArgs = append(scpArgs, "-p", "-r")
        
        // Add source and destination
        scpArgs = append(scpArgs, src, fmt.Sprintf("%s@%s:%s/", sr.SshUser, sr.SshHost, remoteTmpDir))
        
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
            fmt.Printf("Added contents of %s to %s on %s (preserving attributes)\n", src, dest, sr.SshHost)
        } else {
            fmt.Printf("Copied %s to %s on %s (preserving attributes)\n", src, dest, sr.SshHost)
        }
    }
    
    return nil
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

func ParseAndRunDockerfile(dockerfilePath string, runner Runner, predefinedArgs map[string]string) {
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

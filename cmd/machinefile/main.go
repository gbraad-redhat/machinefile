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
	"time"
)

type Runner interface {
	RunCommand(command string, userName string, envVars map[string]string) error
	CopyFile(srcPattern, dest string, isAdd bool) error
}

type LocalRunner struct {
	baseDir string
}

type SSHRunner struct {
	baseDir    string
	sshHost    string
	sshUser    string
	sshKeyPath string
}

func (lr *LocalRunner) RunCommand(command string, userName string, envVars map[string]string) error {
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
				copyErr = exec.Command("bash", "-c", fmt.Sprintf("cp -r %s/* %s/", src, dest)).Run()
			} else {
				copyErr = exec.Command("cp", "-r", src, dest).Run()
			}
		} else {
			copyErr = exec.Command("cp", src, dest).Run()
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

func (sr *SSHRunner) RunCommand(command string, userName string, envVars map[string]string) error {
	sshCommand := command
	
	// Add environment variables to command
	if len(envVars) > 0 {
		envPrefix := ""
		for key, value := range envVars {
			envPrefix += fmt.Sprintf("%s=%s ", key, value)
		}
		sshCommand = envPrefix + sshCommand
	}
	
	// Handle user switching
	if userName != "" {
		sshCommand = fmt.Sprintf("sudo -u %s bash -c '%s'", userName, sshCommand)
	}
	
	// Prepare the SSH command
	var cmd *exec.Cmd
	if sr.sshKeyPath != "" {
		cmd = exec.Command("ssh", "-i", sr.sshKeyPath, fmt.Sprintf("%s@%s", sr.sshUser, sr.sshHost), sshCommand)
	} else {
		cmd = exec.Command("ssh", fmt.Sprintf("%s@%s", sr.sshUser, sr.sshHost), sshCommand)
	}
	
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
	
	// Use local glob to find matching files
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
		
		// Create temp directory for file transfers if needed
		remoteTmpDir := fmt.Sprintf("/tmp/dockerfile-run-%d", time.Now().UnixNano())
		err = sr.RunCommand(fmt.Sprintf("mkdir -p %s", remoteTmpDir), "", nil)
		if err != nil {
			return err
		}
		
		// Copy file to remote host using scp
		var scpCmd *exec.Cmd
		if sr.sshKeyPath != "" {
			scpCmd = exec.Command("scp", "-i", sr.sshKeyPath, "-r", src, fmt.Sprintf("%s@%s:%s/", sr.sshUser, sr.sshHost, remoteTmpDir))
		} else {
			scpCmd = exec.Command("scp", "-r", src, fmt.Sprintf("%s@%s:%s/", sr.sshUser, sr.sshHost, remoteTmpDir))
		}
		
		scpCmd.Stdout = os.Stdout
		scpCmd.Stderr = os.Stderr
		
		if err := scpCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying file to remote host: %v\n", err)
			return err
		}
		
		// Move file to final destination on remote host
		srcBase := filepath.Base(src)
		remoteSrc := filepath.Join(remoteTmpDir, srcBase)
		
		var mvCommand string
		if srcInfo.IsDir() && isAdd {
			// For ADD with directories, create destination and copy contents
			mvCommand = fmt.Sprintf("mkdir -p %s && cp -r %s/* %s/ && rm -rf %s", dest, remoteSrc, dest, remoteTmpDir)
		} else {
			// For COPY or ADD with files, just copy the file/directory
			mvCommand = fmt.Sprintf("mkdir -p $(dirname %s) && cp -r %s %s && rm -rf %s", dest, remoteSrc, dest, remoteTmpDir)
		}
		
		if err := sr.RunCommand(mvCommand, "", nil); err != nil {
			return err
		}
		
		if isAdd {
			fmt.Printf("Added contents of %s to %s on %s\n", src, dest, sr.sshHost)
		} else {
			fmt.Printf("Copied %s to %s on %s\n", src, dest, sr.sshHost)
		}
	}
	
	return nil
}

func parseAndRunDockerfile(dockerfilePath string, runner Runner) {
	file, err := os.Open(dockerfilePath)
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
					srcPattern, dest := parts[0], parts[1]
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
					srcPattern, dest := parts[0], parts[1]
					if err := runner.CopyFile(srcPattern, dest, true); err != nil {
						os.Exit(1)
					}
				} else {
					fmt.Fprintf(os.Stderr, "Invalid ADD command: %s\n", line)
					os.Exit(1)
				}
			} else if strings.HasPrefix(line, "USER ") {
				currentUser = strings.TrimPrefix(line, "USER ")
				fmt.Printf("Switching to user: %s\n", currentUser)
				
				// For local execution, verify user exists
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
	// Command-line flags
	remote := flag.Bool("remote", false, "Run commands on a remote host over SSH")
	sshHost := flag.String("host", "", "SSH host (required if remote is true)")
	sshUser := flag.String("user", "", "SSH user (defaults to current user if remote is true)")
	sshKeyPath := flag.String("key", "", "Path to SSH private key (optional)")
	
	flag.Parse()
	
	args := flag.Args()
	if len(args) != 2 {
		fmt.Printf("Usage: %s [options] <Dockerfile path> <context>\n", os.Args[0])
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}
	
	dockerfilePath := args[0]
	context := args[1]
	
	// Setup the appropriate runner
	var runner Runner
	
	if *remote {
		if *sshHost == "" {
			fmt.Fprintf(os.Stderr, "Error: SSH host is required for remote execution\n")
			os.Exit(1)
		}
		
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
			baseDir:    context,
			sshHost:    *sshHost,
			sshUser:    sshUserName,
			sshKeyPath: *sshKeyPath,
		}
		
		fmt.Printf("Running on remote host %s as user %s\n", *sshHost, sshUserName)
	} else {
		runner = &LocalRunner{
			baseDir: context,
		}
		fmt.Println("Running locally")
	}
	
	parseAndRunDockerfile(dockerfilePath, runner)
}

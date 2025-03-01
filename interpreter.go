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

func runCommand(command string, userName string) {
	var cmd *exec.Cmd
	if userName != "" {
		cmd = exec.Command("sudo", "-u", userName, "sh", "-c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running command: %v\n", err)
		os.Exit(1)
	}
}

func copyFile(src, dest, baseDir string) {
	srcPath := filepath.Join(baseDir, src)
	srcPath = filepath.Clean(srcPath)
	dest = filepath.Clean(dest)

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error stating source file: %v\n", err)
		os.Exit(1)
	}

	if srcInfo.IsDir() {
		err = exec.Command("cp", "-r", srcPath, dest).Run()
	} else {
		err = exec.Command("cp", srcPath, dest).Run()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error copying file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Copied %s to %s\n", srcPath, dest)
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

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "RUN ") {
				command := strings.TrimPrefix(line, "RUN ")
				fmt.Printf("Running: %s\n", command)
				runCommand(command, currentUser)
			} else if strings.HasPrefix(line, "COPY ") {
				parts := strings.Fields(line)
				if len(parts) == 3 {
					src, dest := parts[1], parts[2]
					copyFile(src, dest, baseDir)
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
			} else {
				fmt.Printf("Unsupported command: %s\n", line)
			}
		}
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

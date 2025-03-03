package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PodmanRunner struct {
	BaseDir       string
	ContainerName string
	ConnectionName string // Podman connection name
	PodmanBinary  string  // Path to Podman binary
}

func (pr *PodmanRunner) RunCommand(command string, userName string, envVars map[string]string) error {
	expandedCommand := expandVariables(command, envVars)
	podmanCommand := []string{"exec", pr.ContainerName, "sh", "-c", expandedCommand}

	if len(envVars) > 0 {
		envPrefix := ""
		for key, value := range envVars {
			envPrefix += fmt.Sprintf("%s=%s ", key, value)
		}
		podmanCommand = append([]string{"exec", pr.ContainerName, "sh", "-c", envPrefix + expandedCommand})
	}
	
	if userName != "" {
		podmanCommand = append([]string{"exec", "--user", userName, pr.ContainerName, "sh", "-c", expandedCommand})
	}

	cmd := exec.Command(pr.getPodmanCommand(), podmanCommand...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	fmt.Printf("Executing command in container: %s\n", strings.Join(podmanCommand, " "))
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running command in container: %s, %v\n", command, err)
		return err
	}
	return nil
}

func (pr *PodmanRunner) CopyFile(srcPattern, dest string, isAdd bool) error {
	srcPattern = filepath.Join(pr.BaseDir, srcPattern)
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
		cmd := exec.Command(pr.getPodmanCommand(), "cp", src, fmt.Sprintf("%s:%s", pr.ContainerName, dest))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		fmt.Printf("Copying file to container: %s\n", src)
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error copying file to container: %s, %v\n", src, err)
			return err
		}
	}
	return nil
}

func (pr *PodmanRunner) getPodmanCommand() string {
	command := pr.PodmanBinary
	if pr.ConnectionName != "" {
		command = fmt.Sprintf("%s --connection=%s", pr.PodmanBinary, pr.ConnectionName)
	}
	return command
}

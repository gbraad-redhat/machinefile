package internal

import (
	"os"
	"fmt"
	"os/exec"
	"path/filepath"
)

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

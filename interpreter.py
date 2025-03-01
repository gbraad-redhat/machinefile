#!/bin/env python3
import os
import shutil
import subprocess
import sys

def run_command(command, user=None):
    """Run a shell command and print its output."""
    try:
        if user:
            command = f"sudo -u {user} {command}"
        result = subprocess.run(command, shell=True, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        print(result.stdout.decode())
    except subprocess.CalledProcessError as e:
        print(e.stderr.decode(), file=sys.stderr)
        sys.exit(e.returncode)

def copy_file(src, dest):
    """Copy a file or directory from src to dest."""
    try:
        if os.path.isdir(src):
            shutil.copytree(src, dest)
        else:
            shutil.copy2(src, dest)
        print(f"Copied {src} to {dest}")
    except Exception as e:
        print(f"Error copying {src} to {dest}: {e}", file=sys.stderr)
        sys.exit(1)

def parse_and_run_dockerfile(filepath):
    """Parse a Dockerfile and run each command on the local machine."""
    current_user = None
    with open(filepath, 'r') as file:
        for line in file:
            line = line.strip()
            if line and not line.startswith('#'):
                if line.startswith('RUN '):
                    command = line[4:]
                    print(f"Running: {command}")
                    run_command(command, user=current_user)
                elif line.startswith('COPY '):
                    parts = line.split()
                    if len(parts) == 3:
                        src, dest = parts[1], parts[2]
                        copy_file(src, dest)
                    else:
                        print(f"Invalid COPY command: {line}", file=sys.stderr)
                        sys.exit(1)
                elif line.startswith('USER '):
                    current_user = line[5:]
                    print(f"Switching to user: {current_user}")
                else:
                    print(f"Unsupported command: {line}")

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <Dockerfile path>")
        sys.exit(1)
    
    dockerfile_path = sys.argv[1]
    parse_and_run_dockerfile(dockerfile_path)

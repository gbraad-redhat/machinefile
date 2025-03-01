Machinefile
===========

A simple `Dockerfile`/`Containerfile` executor to set up the local machine


## How It Works

The Machinefile executor tool parses the Dockerfile and executes the commands on the host system. The tool is distributed as a pre-compiled binary for Linux (amd64 and arm64).


## Usage

> [!NOTE]
> This should ideally be run as `root`.

```bash
$ ./machinefile test/Machinefile test
```

To target a remote machine, you have to set up remote keys:

```bash
$ ./machinefile -host [targetmachine] -user root test/Machinefile test
```


## Supported commands

This action supports these Dockerfile commands:

  - `RUN`: Execute commands
  - `COPY`: Copy files from context to a specific location
  - `ADD`: Similar to COPY, but with additional features
  - `USER`: Switch to different user
  - `ENV`: Set environment variables
  - `ARG`: Define build-time variables


## Command line arguments

```bash
# Single ARG
./machinefile --arg=USER="runner" test/Machinefile test

# Multiple ARGs
./machinefile --arg=USER="runner" --arg=VERSION="1.0" test/Machinefile test

# ARGs without quotes (if value doesn't contain spaces)
./machinefile --arg=USER=runner test/Machinefile test
```


## Author

| [!["Gerard Braad"](http://gravatar.com/avatar/e466994eea3c2a1672564e45aca844d0.png?s=60)](http://gbraad.nl "Gerard Braad <me@gbraad.nl>") |
|---|
| [@gbraad](https://gbraad.nl/social) |


Machinefile
===========

[![Machinefile build](https://github.com/gbraad-redhat/machinefile/actions/workflows/build-process.yml/badge.svg)](https://github.com/gbraad-redhat/machinefile/actions/workflows/build-process.yml) [![Machinefile test](https://github.com/gbraad-actions/machinefile-executor-action/actions/workflows/build-process.yml/badge.svg)](https://github.com/gbraad-actions/machinefile-executor-action/actions/workflows/build-process.yml)

A simple executor that allows you to run `Dockerfile`/`Containerfile` commands directly on the host system without using Docker, Podman or any other container engine. It's useful for executing build commands in a predictable environment or setting up development tools. The Machinefile executor tool parses the Dockerfile and executes the commands on the local or remote host system. 


## Supported commands

The executor supports the followuing `Dockerfile` commands:

  - `RUN`: Execute commands
  - `COPY`: Copy files from context to a specific location
  - `ADD`: Similar to COPY, but with additional features
  - `USER`: Switch to different user
  - `ENV`: Set environment variables
  - `ARG`: Define build-time variables


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

### Passing arguments

```bash
# Single ARG
./machinefile --arg=USER="runner" test/Machinefile test

# Multiple ARGs
./machinefile --arg=USER="runner" --arg=VERSION="1.0" test/Machinefile test

# ARGs without quotes (if value doesn't contain spaces)
./machinefile --arg=USER=runner test/Machinefile test
```


## GitHub Action

To incorporate this in your build process, you can use the [Machinfile executor](https://github.com/gbraad-actions/machinefile-executor-action) GitHub Action.

```yaml
    - name: Run Dockerfile commands
      uses: gbraad-actions/machinefile-executor-action@v1
      with:
        containerfile: 'containers/Containerfile-devtools'
        context: '.'
        arguments: --arg=USER=gbraad
```


## Author

| [!["Gerard Braad"](http://gravatar.com/avatar/e466994eea3c2a1672564e45aca844d0.png?s=60)](http://gbraad.nl "Gerard Braad <me@gbraad.nl>") |
|---|
| [@gbraad](https://gbraad.nl/social) |


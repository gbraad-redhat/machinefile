Machinefile
===========

A simple `Dockerfile`/`Containerfile` interpreter to set up the local machine


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


## Author

| [!["Gerard Braad"](http://gravatar.com/avatar/e466994eea3c2a1672564e45aca844d0.png?s=60)](http://gbraad.nl "Gerard Braad <me@gbraad.nl>") |
|---|
| [@gbraad](https://gbraad.nl/social) |


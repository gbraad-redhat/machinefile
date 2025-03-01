Machinefile
===========

A simple `Dockerfile`/`Containerfile` interpreter to set up the local machine


## Usage

> [!NOTE]
> This should ideally be run as `root`.

### Python

```bash
$ ./interpreter.py test/Containerfile
```

### Go

```
$ go build -o machinefile interpreter.go
$ cd test
$ ../machinefile Containerfile
```


## Author

| [!["Gerard Braad"](http://gravatar.com/avatar/e466994eea3c2a1672564e45aca844d0.png?s=60)](http://gbraad.nl "Gerard Braad <me@gbraad.nl>") |
|---|
| [@gbraad](https://gbraad.nl/social) |


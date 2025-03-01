Machinefile
===========

A simple `Dockerfile`/`Containerfile` interpreter to set up the local machine


## Usage

> [!NOTE]
> This should ideally be run as `root`.

```
$ go build -o machinefile interpreter.go
$ ./machinefile test/Machinefile test
```

### Test result

```bash
[root@wint14-devsys-gosys Machinefile]# go build -o machinefile interpreter.go
[root@wint14-devsys-gosys Machinefile]# ./machinefile test/Machinefile test
Unsupported command: FROM scratch
Running: whoami
root
Running: echo hello
hello
Copied hello to /tmp/hello
Running: cat /tmp/hello
Hello, World!
Switching to user: gbraad
Running: whoami
gbraad
[root@wint14-devsys-gosys Machinefile]#
```


## Author

| [!["Gerard Braad"](http://gravatar.com/avatar/e466994eea3c2a1672564e45aca844d0.png?s=60)](http://gbraad.nl "Gerard Braad <me@gbraad.nl>") |
|---|
| [@gbraad](https://gbraad.nl/social) |


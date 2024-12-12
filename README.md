

`irisctl` is a command line tool to make it easier to work Iris.  It has two main command groups:
  - Iris API commands
  - Non-API commands for checking and analyzing Iris data

To build `irisctl` from source, you need the standard developments
tools (make, git) and also the Go language compiler on your machine.
```
$ cd /to/your/work/directory
$ git clone https://github.com/dioptra-io/irisctl.git
$ cd irisctl
$ make
$ ./irisctl -h
```

`irisctl` reads your Iris's user name from the file
`$HOME/.iris/credentials` (e.g., joe.blow@lip6.fr) and prompts you
for your password (unless the `IRIS_PASSWORD` environment variable
is set to your password).

There are usage examples in `COOKBOOK.txt`.

Please note that `irisctl` is work-in-progress and currently does
not meet production-quality requirements (e.g., it lacks unit tests).

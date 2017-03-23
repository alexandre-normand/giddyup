giddyup
=======

![giddyup](http://i.imgur.com/8ikUN93.gif)

`giddyup` is a tool that can be used as a `go generate` command (see [main.go](main.go)) or a standalone CLI to manage release versions of a `golang` application. 

How to use
----------
`giddyup` assumes a `version.go` file with a `VERSION` constant such. When executed with the default options, `giddyup` reads the current version and increments the patch-level in `version.go`. Since the file is meant to be managed by `giddyup`, interactions with it should be done using `giddyup` only. To initialize the version, you can run `giddyup --init` in your repository which will generate a `version.go` with the `VERSION` set to `1.0.0`. 

An application can then print its version using the `VERSION` string constant.
Example using [kingpin](https://github.com/alecthomas/kingpin): 

```
kingpin.Version(VERSION)
```

Integration with go generate
----------------------------

`giddyup` itself uses `giddyup` for managing its version. This is done by adding the following line to `main.go`:

```
//go:generate giddyup
```


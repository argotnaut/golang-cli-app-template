# Cmd
This directory contains code for the commands and subcommands available
when running the app binary from the command line


`root.go` contains code for installing and uninstalling the binary on multiple platforms, as well as the definition of the app's root command in the `init()` function, which it exposes the the entry point in `./main.go` through the function `RootCmd()`


It is recommended to make a new `.go` file in this directory for each command. If a command has multiple subcommands, it is recommended to create a subdirectory within this one named after the command, and put the `.go` files for the command and its subcommands in that directory. For example, in an app that allows users to manage network connections by calling `mynetmanager connections list`, `mynetmanager connections add <name>`, and `mynetmanager connections remove <name>` this directory might have the following structure:
```
cmd/
- connections/
  - connections.go
  - add.go
  - remove.go
- root.go
```
In this case, the first line `init()` in `root.go` would look something like:
```
    rootCmd.AddCommand(connections.InitConnectionsCommand())
```
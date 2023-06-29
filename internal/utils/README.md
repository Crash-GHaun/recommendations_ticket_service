# Logger Utility

This package provides a simple logging utility in Go.

## Usage

Import the package as follows:

```go
import "utils"
```

The logger utility provides the following log levels:

```go
const (
	Debug = 1
	Info = 2
	Warning = 3
	Error = 4
)
```

You can set the log level by setting an environment variable "LOG_LEVEL" with the appropriate value.

The function to log messages is `LogPrint(level int, v ...interface{})`. Here's how to use it:

```go
utils.LogPrint(utils.Info, "This is an info message")
```

## Functionality

`LogPrint` takes in two parameters:

1. `level`: An integer representing the log level. If this level is higher or equal to the currently set log level, the log message will be printed. 

2. `v`: Variadic parameter. The first value should be a string which forms the log message. Any subsequent values will be interpolated into the string where there is a corresponding placeholder. 

Example:
```go
utils.LogPrint(utils.Warning, "An error occurred: %v", err)
```

`LogPrint` will check the environment variable "LOG_LEVEL" and set the log level accordingly, defaulting to `Error` if the environment variable is invalid or not set. If the log level is `Error`, `LogPrint` will exit the program after logging the message.
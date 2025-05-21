# golangci-lint run fails

Running `golangci-lint run ./...` results in a panic with the message:
```
* can't run linter goanalysis_metalinter: goanalysis_metalinter: buildir: package "slices" ... Cannot range over: func(yield func(E) bool)
```
(as seen in `/tmp/lint.log`)
This prevents linting from completing successfully and suggests a misconfiguration or incompatible linter version. According to the Go coding best practices, code should be linted before merging.

Investigate the lint configuration or update golangci-lint so that lint checks run cleanly.

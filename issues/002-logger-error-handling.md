# Logger initialization ignores errors

`internal/proxy/proxy.go` creates the zap logger but discards potential errors:
```
44:    logger, _ := zap.NewProduction()
46:            logger, _ = zap.NewDevelopment()
```
The Go coding best practices specify that errors should always be checked and handled【F:.cursor/rules/go-coding-best-practices.mdc†L16-L21】. Failure to handle logger initialization errors may hide configuration problems.

Replace the ignored assignments with proper error handling and propagate or log failures appropriately.

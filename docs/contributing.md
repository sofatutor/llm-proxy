[Home]({{ site.baseurl }}/) | [Features]({{ site.baseurl }}/features) | [Screenshots]({{ site.baseurl }}/screenshots) | [Quickstart]({{ site.baseurl }}/quickstart) | [CLI Reference]({{ site.baseurl }}/cli-reference) | [Architecture]({{ site.baseurl }}/architecture) | [Contributing]({{ site.baseurl }}/contributing) | [Coverage]({{ site.baseurl }}/coverage/) | [Roadmap](https://github.com/sofatutor/llm-proxy/blob/main/PLAN.md)

## Contributing

Thanks for your interest in contributing! Start here:

- Read `CONTRIBUTING.md` and `AGENTS.md` for workflow and quality gates
- Open a PR with small, focused commits
- Tests must pass with `-race`; coverage ≥ 90%
- Run `make lint` and format code before pushing

Helpful links:
- Repo: `https://github.com/sofatutor/llm-proxy`
- Issues: look for `good first issue`
- Architecture: `docs/architecture.md`
- CLI Reference: `docs/cli-reference.md`

### Local validation

```bash
make lint
go test -v -race ./...
go test -v -race -parallel=4 -coverprofile=coverage_ci.txt -covermode=atomic -coverpkg=./internal/... ./...
go tool cover -func=coverage_ci.txt | tail -n 1
```



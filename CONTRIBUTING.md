# Contributing to Harbor

Welcome! We're glad you want to contribute.

## Before You Start

By submitting a pull request, you agree to our [Contributor License Agreement](CLA.md).
This ensures the project can be sustainably maintained and commercially licensed alongside
the open-source Apache 2.0 release.

## Ways to Contribute

- **Bug reports** — Open an issue with reproduction steps
- **Connectors** — Write a new connector using `harbor scaffold`
- **Schema contributions** — Share your `harbor_learn_schema` learnings
- **Code improvements** — Fix bugs, improve performance, add tests
- **Documentation** — Improve README, tutorials, or inline docs

## Development Setup

```bash
git clone https://github.com/oSEAItic/harbor.git
cd harbor
go build ./cmd/harbor
./harbor doctor          # verify your environment
```

## Connector Development

```bash
harbor scaffold my-connector --lang typescript
cd my-connector
# edit src/index.ts
harbor build
harbor get my-connector.resource -p key=value
```

See the [TypeScript SDK](https://www.npmjs.com/package/@AgenJay/harbor-sdk) for the connector API.

## Pull Request Guidelines

1. Keep PRs focused — one feature or fix per PR
2. Include tests for new functionality when applicable
3. Run `go test ./...` before submitting
4. Follow existing code style and patterns

## License

All contributions are licensed under [Apache 2.0](LICENSE), subject to the [CLA](CLA.md).

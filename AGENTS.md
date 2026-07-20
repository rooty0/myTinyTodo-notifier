# AGENTS.md

## Goal

This repository is written in Go. When making changes, prioritize correctness, simplicity, maintainability, and idiomatic Go.

Prefer small, focused changes over broad rewrites. Do not introduce abstractions unless they clearly reduce complexity or duplication.

## General Rules

* Read the existing code before making changes.
* Follow the style and patterns already used in the repository.
* Keep changes minimal and directly related to the task.
* Do not rename exported APIs, packages, files, or public structs unless required.
* Do not change behavior unrelated to the requested work.
* Do not add new dependencies unless there is a strong reason.
* Prefer clear, boring code over clever code.
* Keep functions small and easy to reason about.
* Favor explicit control flow over hidden magic.

## Go Style

Follow idiomatic Go practices:

* Use `gofmt` / `go fmt`.
* Use `goimports` if available.
* Prefer short variable names for local, obvious values.
* Prefer descriptive names for exported symbols, complex values, and package-level identifiers.
* Keep package names short, lowercase, and meaningful.
* Avoid stutter in exported names. Prefer `queue.Manager` over `queue.QueueManager`.
* Return early to reduce nesting.
* Keep interfaces small.
* Define interfaces at the consumer side when possible.
* Do not use unnecessary interfaces for a single implementation.
* Do not use global mutable state unless the existing code already relies on it.
* Avoid panics in library code unless the situation is truly unrecoverable.
* Prefer composition over inheritance-style designs.

## Error Handling

* Always handle errors explicitly.
* Do not ignore errors unless it is intentional and documented.
* Wrap errors with useful context using `fmt.Errorf("...: %w", err)`.
* Do not log and return the same error unless there is a specific reason.
* Prefer sentinel errors or typed errors only when callers need to inspect them.
* Keep error messages lowercase and without trailing punctuation.

Example:

```go
if err := store.Save(ctx, item); err != nil {
	return fmt.Errorf("save item %q: %w", item.Name, err)
}
```

## Context Usage

* Pass `context.Context` as the first argument when a function performs I/O, blocking work, external calls, or long-running operations.
* Do not store contexts in structs unless the existing design requires it.
* Do not pass `nil` contexts.
* Respect cancellation and deadlines.
* Avoid creating background contexts deep inside business logic.

Preferred:

```go
func (s *Service) Reconcile(ctx context.Context, key string) error {
	// ...
}
```

## Concurrency

* Keep concurrency simple and justified.
* Avoid goroutines unless they are clearly needed.
* Make goroutine lifetimes obvious.
* Ensure goroutines can exit on context cancellation.
* Avoid data races.
* Use channels for coordination, not as a replacement for simple function calls.
* Prefer `sync.Mutex` when shared state needs protection.
* Prefer `errgroup` only if it is already used or clearly improves the code.

## Logging

* Follow the logging style already used in the repository.
* Log useful operational context.
* Do not log secrets, tokens, credentials, or sensitive values.
* Avoid excessive logs in hot paths.
* Prefer structured logging if the repository already uses it.

## Testing

Add or update tests for behavior changes.

Prefer table-driven tests when they make the test clearer.

Tests should cover:

* Normal behavior.
* Edge cases.
* Error paths.
* Regression cases for bugs being fixed.

Keep tests deterministic. Avoid sleeps when possible. Prefer fake clocks, fake clients, temporary directories, or controlled synchronization.

Example:

```go
func TestParseName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid name",
			input: "example",
			want:  "example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

## Mocks and Fakes

* Prefer small hand-written fakes over heavy mocking when practical.
* Mock behavior at boundaries, not internal implementation details.
* Do not over-specify call order unless ordering is part of the behavior.
* Keep tests focused on observable behavior.

## Dependencies

Before adding a dependency:

* Check whether the standard library is sufficient.
* Check whether the repository already has a suitable dependency.
* Avoid large dependencies for small helpers.
* Avoid adding dependencies that make builds slower or introduce unnecessary risk.

When modifying dependencies:

```bash
go mod tidy
```

Review `go.mod` and `go.sum` changes carefully.

## Generated Code

* Do not edit generated files manually unless explicitly requested.
* If generated files need changes, update the source and regenerate them using the repository’s documented commands.
* Preserve generated file headers.

## Repository Commands

Use the repository’s existing commands when available.

Common Go commands:

```bash
go fmt ./...
go test ./...
go vet ./...
go mod tidy
```

If the repository has a `Makefile`, prefer existing targets such as:

```bash
make test
make lint
make generate
```

Do not invent new build or test commands unless necessary.

## Linting

When linting is configured, use the repository’s existing linter configuration.

Common command:

```bash
golangci-lint run ./...
```

Do not silence linter warnings without understanding the issue. Prefer fixing the code.

## API and Compatibility

* Preserve public API compatibility unless the task requires changing it.
* Be careful with exported types, methods, constants, struct fields, JSON tags, protobuf fields, and CLI flags.
* Do not change serialized formats unless explicitly required.
* Keep backward compatibility in mind for config files, environment variables, and command-line arguments.

## Performance

* Do not optimize prematurely.
* Prefer readable code first.
* Avoid unnecessary allocations in hot paths when easy to do so.
* Avoid repeated expensive calls inside loops.
* Use benchmarks only when performance is part of the task.

## Security

* Do not commit secrets, credentials, tokens, private keys, kubeconfigs, or certificates.
* Do not print sensitive data in logs or errors.
* Validate external input.
* Be careful when constructing shell commands, file paths, URLs, or SQL queries.
* Prefer safe standard-library APIs.

## Comments and Documentation

* Add comments for non-obvious behavior.
* Do not comment obvious code.
* Exported symbols should have comments when required by linting.
* Keep comments accurate when changing behavior.

Good comments explain why, not just what.

## Change Review Checklist

Before finishing:

* Code is formatted.
* Tests were added or updated when needed.
* Existing tests pass, or failures are explained.
* Errors include useful context.
* No unrelated changes were made.
* No unnecessary dependencies were added.
* Public API compatibility was considered.
* Generated files were handled correctly.
* Documentation was updated if behavior changed.

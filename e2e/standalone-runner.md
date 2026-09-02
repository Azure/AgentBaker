## Why

- Safer concurrency. `testing.T.Fatal`, `FailNow`, `SkipNow`, and Testify `require` are not safe to call from goroutines. Returning errors to the runner lets us add safe concurrency and reduce E2E time and cost.
- Control over the scenario lifecycle. The runner can manage retries, mark flaky scenarios during release validation, enforce individual timeouts, recover panics, propagate cancellation, and run cleanup.
- One documented interface for humans and agents. Today configuration is split across Go test arguments and global variables, `gotestsum` arguments, and environment variables. The CLI puts it in one place.
- Separation of unit and E2E results. The pipeline can report more than 300 tests because it mixes E2E scenarios, unit tests, and nested checks. The standalone runner reports selected E2E scenarios separately.
- Fewer workarounds to make `go test` usable for long-running workflows, such as adding timestamps to test logs.
- Direct control over logging and ADO output. Today the pipeline downloads `gotestsum` on each run to convert Go test output, and Go test timeouts can discard buffered logs. Owning the output format lets us remove these workarounds, choose when to show logs from passed and failed scenarios, collapse or expand scenario output in ADO, attach logs, and add extension views.
- Faster unit-test failures. Unit tests run before the E2E suite instead of failing alongside long-running scenarios.
- A clean place to prepare shared infrastructure before the suite and shut it down after the suite.

## Related cleanup

- Stop passing `*testing.T` through scenario helpers.
- Return errors to the caller instead of letting any helper fail the scenario directly.
- Stop calling `t.Helper` throughout E2E helpers.

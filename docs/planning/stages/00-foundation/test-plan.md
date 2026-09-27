# Stage 00 test plan

No product tests. The stage is complete when the repository builds.

## Verified

- `make test` — `go test ./...` passes with no test files yet.
- `make build` — `go build -o bin/ambit ./cmd/ambit` produces a binary whose `main` is empty.

## Not in this stage

- The `.gitignore` integration gate. That is stage 01 task 01.9, because it has to run a
  real `ambit init`.
- Frontend tests. The SPA is still the pipeline template.

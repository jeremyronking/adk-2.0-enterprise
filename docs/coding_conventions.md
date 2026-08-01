# Enterprise Coding Conventions

This document outlines the coding standards, package structuring guidelines, and ADK 2.0 tool patterns for building microservices in `adk-2.0-enterprise`.

---

## 1. Directory & Package Structuring

- **`cmd/<service>/`**: Entry points containing executable `main.go` routines. Responsibilities are strictly limited to flag parsing, environment configuration, ADK model initialization, and `launcher.Execute()`.
- **`agents/<service>/`**: Domain agents and sub-agents. Contains agent definition constructors (`New`), sub-agent registrations, and custom function tools.
- **`internal/<domain>/`**: Core domain logic, external HTTP API clients (e.g. `openmeteo`), and service registries (`registry`). Must remain decoupled from ADK dependencies so domain logic can be tested in isolation.

---

## 2. Sub-Agent Packaging & Export Constants

- Every sub-agent package must export `Name` and `Description` constants or package variables:
  ```go
  package timeagent

  const Name = "time_agent"
  const Description = "Reports current local date, time, and timezone for a named city or place."
  ```
- The `Name` must be a valid Go identifier as `agenttool.New` exposes it directly to the parent orchestrator LLM.

---

## 3. Trade Agent Patterns (`agents/construction/`)

For multi-phase workflow domains (such as construction trades), use generic factory constructors rather than duplicate boilerplate:
- **`NewTradeTool(toolName, phase)`**: Creates a phase-specific `functiontool` handler.
- **`NewTradeAgent(name, phase, toolName, model)`**: Instantiates an `llmagent` sub-agent configured with phase-specific instructions and tool bindings.

---

## 4. Function Tool Patterns (`functiontool`)

### Struct Field Annotations
- All input argument structs MUST include `json` and `jsonschema` tags describing requirements and concrete examples for the LLM:
  ```go
  type TradeArgs struct {
      StructureType string `json:"structure_type" jsonschema:"The type of structure to operate on (e.g., \"residential home\", \"commercial tower\", \"bridge\")."`
  }
  ```

### Soft Error Handling Strategy
- Tools MUST return operational or lookup errors inside the result struct (`Status: "error"`, `ErrorMessage: ...`) with a `nil` Go error:
  ```go
  func handleTrade(ctx agent.Context, args TradeArgs) (TradeResult, error) {
      if args.StructureType == "" {
          return TradeResult{Status: "error", ErrorMessage: "structure_type is required"}, nil
      }
      return TradeResult{Status: "success", Message: "Completed trade task"}, nil
  }
  ```
- **Rationale**: Returning a `nil` Go error prevents turn cancellation, allowing the LLM to inspect the error message and recover gracefully during turn execution.

---

## 5. Quality Assurance Gate

Always execute the workspace quality gate before committing modifications:
```bash
make check
```
`make check` executes code formatting (`gofmt`), static analysis (`go vet`), and unit test suites across all microservices (`go test ./...`).

# Runbook: using ambit in your app

## Get ambit onto your machine

From a checkout of this repo (needs Go and Node):

    make install          # builds with the UI embedded, installs to ~/.local/bin/ambit

Make sure `~/.local/bin` is on your `PATH` (`PREFIX=/some/dir make install` installs under
`/some/dir/bin`). Check with `ambit version`.

After a release is published, `npx ambit <command>` downloads, checksum-verifies and caches
the binary, then runs it ([release runbook](release.md)). `npx` is not available until
then, and the npm name is unchecked.

## In your app repo

    cd your-app
    ambit init            # scaffolds .arch/ (does not install a git hook)
    ambit start           # serves UI + API on 127.0.0.1:8080

Open http://127.0.0.1:8080. `ambit start` reads `.arch/` from the current directory and
takes `--addr <host:port>` (loopback only). It prints a warning if `.arch/local.json`,
`.arch/.cache/` or `.arch/.proposals/` are not gitignored.

## Connect your coding harness (MCP)

`ambit mcp [dir]` serves the MCP tools over stdio; `dir` defaults to the current directory
and must contain `.arch/`. It is independent of `ambit start`. Tools:
[mcp-tools.md](../contracts/mcp-tools.md).

Claude Code, `.mcp.json` in the app repo root:

    {
      "mcpServers": {
        "ambit": { "command": "ambit", "args": ["mcp"] }
      }
    }

Cursor, `.cursor/mcp.json`:

    {
      "mcpServers": {
        "ambit": { "command": "ambit", "args": ["mcp"] }
      }
    }

If the harness does not start in the repo root, pass the directory:
`"args": ["mcp", "/absolute/path/to/your-app"]`. Before the npm package is published, use
the installed `ambit`; afterwards `"command": "npx", "args": ["ambit", "mcp"]` also works
(the shim keeps stdout clean for JSON-RPC). The exact config key names are each harness's;
check their docs if yours differs.

Authoring tools (`seed_model`, `create_node`, ...) only stage proposals in
`.arch/.proposals/`; nothing changes the model until you accept it in the UI.

## Enforcement: `ambit check` and the hook

`ambit check` runs in the repo, diffs files changed in git, and prints a scope report for
the currently assigned node (assignment lives in `.arch/local.json`). It exits non-zero on
errors (missing `.arch`, git or glob failures); warnings (for example un-ignored paths or a
dangling assignment) go to stderr and do not change the exit code.

`ambit hook install` writes `.git/hooks/pre-commit`, which runs `ambit check` using the
absolute path of the binary that installed it. It refuses to overwrite an existing hook.
If you installed via `npx`, that path is the cached binary under the npm cache version
directory, so reinstall the hook after upgrading.

## What to commit in `.arch/`

Commit: `.arch/nodes/*.json`, `.arch/nodes/*.md`, `.arch/index.json`, `.arch/config.json`.

Do not commit (machine-local or derived, see
[arch-model-format.md](../contracts/arch-model-format.md)): `.arch/local.json`,
`.arch/.cache/`, `.arch/.proposals/`. Add them to the app's `.gitignore`:

    .arch/local.json
    .arch/.cache/
    .arch/.proposals/

## Dev loop (working on ambit's UI)

    ambit start                       # in the app repo (or `make dev-build && bin/ambit start`)
    cd frontend && npm ci && npm run dev    # Vite; proxies the API to 127.0.0.1:8080

`make build` embeds the UI into the binary. Plain `go build`/`make dev-build` produce an
API-only binary; without the bundle, `GET /` returns a message naming `make build`.

## First session

1. `ambit init`, `ambit start`, open the UI.
2. Create a root node, then add child nodes under it (sidebar).
3. Group related nodes (flat-first canvas, [decision 0010](../decisions/0010-flat-first-groups-and-summary-nodes.md)).
   Grouping is staged as a proposal; accept it in the review panel.
4. Assign a node to an agent: select the node, then in the inspector's "Agent assignment"
   section press "Assign agent to this node" (an "Agent" chip marks it on the canvas). It is
   stored in `.arch/local.json` and does not change the node's status. "Clear assignment"
   removes it. The same is available as `PUT`/`DELETE /assignment`; the UI also reflects
   assignments changed outside it.
5. In the harness, the agent calls `get_context` for its brief, `check_scope` before
   writing, and `update_node_status` when done. Run `ambit check` before committing.
6. Review agent proposals in the UI and accept or reject each operation.

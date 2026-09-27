# ambit canvas

The architect SPA. `ambit start` serves the local HTTP API. This app stays on the Vite dev server until stage 06 embeds it. The client calls relative paths, and Vite proxies them to the API.

## Run

From the repository root:

```bash
go run ./cmd/ambit init
go run ./cmd/ambit start
```

`ambit start` listens on `127.0.0.1:8080` by default. It accepts only a loopback address.

In `frontend/`:

```bash
npm install
npm run dev
```

Open `http://localhost:5173`. Reload a drill-down at `http://localhost:5173/node/<id>` against Vite, not the Go process.

The dev server proxies `/levels`, `/nodes`, `/integrity`, `/relationships`, `/assignment`, `/layout`, and `/events` to `http://127.0.0.1:8080`.

`npm test` runs the layout-cache key and scope-preview checks. `npm run typecheck` typechecks the SPA.

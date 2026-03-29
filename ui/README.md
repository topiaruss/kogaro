# Kogaro UI (Desktop)

`ui/` contains the desktop application for Kogaro, built with Wails (Go backend + Svelte frontend).

## What this component is

- Desktop incident investigation and remediation workflow for Kogaro diagnostics.
- Frontend source: `ui/frontend/`
- Desktop app config: `ui/wails.json`
- Local build automation: `ui/Makefile`

## Run in development

From `ui/`:

```bash
wails dev
```

This starts the desktop dev loop with live frontend reload. The Wails browser bridge is also available during development.

## Build the desktop binary

From `ui/`:

```bash
make build
```

Output binary:

- `ui/build/bin/kogaro-ui`

## Important: do not commit built binaries

The repo root may contain a generated binary named `ui/kogaro-ui` from manual/local builds. This is a build artifact and should not be committed.

## Related component: website

The public marketing/docs website lives in `website/` and is deployed separately from the desktop UI.

See:

- `website/README.md`
- `website/Makefile`

# gastown-micro

Minimal tmux + Claude Code loop with hooks.

## What it does

- `gtm init` writes `./.claude/settings.json` with two hooks:
  - `SessionStart` → `bd prime`
  - `Stop` → `gtm hook-stop` (respawns Claude in the same tmux pane)
- `gtm start` launches Claude in a tmux session/window and attaches you to it.
- When Claude exits, the Stop hook restarts it in the same pane.

## Install

```bash
cd /Users/chris/Development/gastown-micro
make build
```

## Usage

```bash
# In your project repo
/path/to/gtm init
/path/to/gtm start
```

Common flags:

```bash
# Custom session name
/path/to/gtm start --session gtm-myproj

# Custom Claude command
/path/to/gtm start --cmd "claude --dangerously-skip-permissions"
```

Stop the session:

```bash
/path/to/gtm stop
```

## Notes

- `gtm hook-stop` must run inside tmux (it uses `TMUX_PANE`).
- The restart command is stored in tmux session env as `GTM_RESTART_CMD`.
- Hooks are written to `./.claude/settings.json` in the repo root.

# Repository Vision & Goals

## Philosophy

This project is a **free, minimal, self‑contained AI agent framework** built for old, low‑power hardware. It embraces:
- **Single binary simplicity** – PicoClaw is one file, no dependency hell.
- **Chat‑first interaction** – everything is controllable from your phone via Telegram.
- **Full lifecycle automation** – setup, start, stop, update, clean, uninstall are just scripts away.
- **Security by design** – sandboxed filesystem, confirmed shell, secrets never leaked.

## Core Goals

1. Provide a drop‑in, ready‑to‑run agent for anyone with a spare laptop and a free Groq account.
2. Enable easy customisation through modular agent personalities and scripts.
3. Maintain reliability: health checks, backups, and clean teardown.
4. Never require a credit card or paid API key.

## Roadmap

- [x] Groq + Telegram integration
- [x] Lifecycle scripts (setup, start, stop, status, clean)
- [x] Sandboxed file operations
- [ ] Add Discord & Slack gateways
- [ ] Implement daily scheduled tasks via cron wrapper
- [ ] Simple encryption tool (shuffle a file’s bytes using a key file)
- [ ] Web dashboard for configuration and logs
- [ ] Community plugins / skills directory

## Maintainer Notes

- The `picoclaw` binary is **never committed**. It is fetched by `setup.sh` from the official GitHub releases.
- Configuration always lives in `config/config.json`; gateways can later be split out.
- Logs are kept in `logs/`, workspace in `workspace/`. Both are gitignored (except `.gitkeep`).
- All scripts assume they are run from the repository root.

## Security Considerations

- The `.env` file must never be committed.
- The default filesystem root is `workspace/`. Changing this exposes your system.
- The shell tool is set to `confirmation_required: true`; this is intentional and should not be overridden lightly.
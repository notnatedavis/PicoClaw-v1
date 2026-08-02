# PicoClaw (v1)

A complete, free, lightweight ai agent framework built around [PicoClaw](https://github.com/sipeed/picoclaw) – the ultra‑fast, single‑binary Go agent. This repository `PicoClaw (v1)` provides  everything needed to run personal assistant on old hardware, controlled via Telegram (or other messengers), powered entirely through free AI models (Ollama)

---

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Project-Structure](#Project-Structure)
- [Launch](#launch)
- [Usage](#usage)
- [Configuration](#Configuration)
- [Troubleshooting](#troubleshooting)
- [Additional-Information](#Additional-Info)

---

## Introduction

PicoClaw acts as the central “brain” connecting a chosen AI model (Groq, Ollama, etc.) to the real world:
- **Communicate** via Telegram, Discord, Slack, and more.
- **Act** through built‑in tools: filesystem, shell, web search.
- **Remember** conversations in a local SQLite database.

This repository provides a fully modular, script‑driven environment. Everything is free, local‑first, and designed for minimal resource usage.

---

## Features

- Ultra low RAM usage ; an old ass laptop with limited RAM is more than enough
- Blazing fast boot ; ready almost instantly
- Single binary deployment ; Download 1 file, make it executable and run it
- Built in agent system ; allows for personalization of unique agents for different tasks
- Integrated tools ; bring foundationals and essentails to build up on or off of
- Built in memory ; uses SQLite to remember conversations & context

---

## Project-Structure

```bash
PicoClaw-v1/
├── binaries/   # selected on setup
│   ├── picoclaw-binary-mac-arm64
│   ├── picoclaw-binary-mac-x86
│   ├── picoclaw-binary-win-arm64.exe
│   └── picoclaw-binary-win-x86.exe
├── config/
│   ├── agents/ # custom agents
│   │   └── assistant.json
│   ├── skills/ # (future) user‑installed skills
│   └── config.json
├── docs/       # philosophy, roadmap, notes
│   ├── RepositoryVisionGoal.md
│   └── ToDo.md
├── logs/       # runtime logs (not tracked)
├── scripts/    # lifecycle management
│   ├── backup-agent-logs.sh
│   ├── clean.sh
│   ├── health-check.sh # *
│   ├── setup-ollama.sh
│   ├── setup.sh        # *
│   ├── start.sh        # *
│   ├── status.sh
│   ├── stop.sh
│   ├── uninstall.sh
│   └── update.sh
├── workspace/ # agent sandbox
│   └── agent-sessions/ # per‑agent
│       └── .gitkeep
├── .env.example
├── .gitignore
└── ReadMe.md   # (You are here , hi !)
```

## Launch

1. **Clone & enter**
   ```bash
   git clone <https://github.com/notnatedavis/PicoClaw-v1.git> && cd PicoClaw-v1
    ```

2. **Set up environment**
    ```bash
    # create new file '.env' clone '.env.example'
    # 
    ```

3. **Run the full setup**
    Open Git Bash (win) / terminal (mac)
    ```bash
   chmod +x scripts/*.sh
   bash scripts/setup.sh
   bash scripts/health-check.sh
   ```

4. **Start the agent gateway**
    ```bash
    bash scripts/start.sh
    bash scripts/status.sh
    ```

5. **Chat with your bot on Telegram**
- Send /start or any message
- Use bash scripts/status.sh to see if it’s alive

---

## Usage 

1. **Chat with PicoClaw on Telegram**
- Send /start or any message eg "Who are you ?"
- Use bash scripts/status.sh to see if it’s alive

## Configuration

- `config/config.json` holds the core , sets specific model, chat gateway, tools skills scripts etc to use. This lives in `/.picoclaw/config.json` by default but using (`PICOCLAW_CONFIG=./config/config.json`) makes it alot easier 

^ make sure this is implemented

- `workspace/` is the placeholder sandbox. PicoClaw reads and writes heres maintaining boundaries (v important)
- `scripts/` is the personal customization of modular functionality like (auto read the news for today from x summarize and return to me) with so much potential
- `.env` holds api keys (dont commit) its included in the .gitignore



## Additional-Info

This portion is for logging or storing notes relevent to the project and its scope.
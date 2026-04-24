# PicoClaw (v1)

A complete, free, lightweight ai agent framework built around [PicoClaw](https://github.com/sipeed/picoclaw) – the ultra‑fast, single‑binary Go agent. This repository `PicoClaw (v1)` provides  everything needed to run personal assistant on old hardware, controlled via Telegram (or other messengers), powered entirely through free AI models (Groq, Ollama, etc.)

--- 

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Project-Structure](#Project-Structure)
- [Usage](#usage)
- [Configuration](#Configuration)
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
├── .env.example
├── .gitignore
├── ReadMe.md # (You are here , hi !)
├── picoclaw # <- (binary, downloaded by setup.sh)
├── config/
│   ├── config.json
│   ├── agents/ # custom agents
│   │   └── assistant.json
│   ├── gateways/ # (future) separate gateway configs
│   └── skills/ # (future) user‑installed skills
├── docs/ # philosophy, roadmap, maintainer notes
│   └── RepositoryVisionGoal.md
├── logs/ # runtime logs (not tracked)
├── scripts/ # lifecycle management
│   ├── setup.sh
│   ├── start.sh
│   ├── stop.sh
│   ├── status.sh
│   ├── update.sh
│   ├── clean.sh
│   ├── uninstall.sh
│   ├── backup-agent-logs.sh
│   ├── health-check.sh
│   └── setup-ollama.sh
└── workspace/ # agent sandbox – all file operations go here
    └── agent-sessions/ # per‑agent isolated directories
```

## Usage 

1. **Clone & enter**
   ```bash
   git clone <your-repo-url> && cd PicoClaw-v1
    Set up environment
    ```

2. **Set up environment**
    ```bash
    cp .env.example .env
    # edit .env with your real API keys (Groq + Telegram)
    ```

3. **Run the full setup**
    ```bash
   scripts/setup.sh
   Start the agent gateway
   ```

4. **Start the agent gateway**
    ```bash
    bash scripts/start.sh

5. **Chat with your bot on Telegram**
- Send /start or any message
- Use bash scripts/status.sh to see if it’s alive

## Configuration

- `config/config.josn` holds the core , sets specific model, chat gateway, tools skills scripts etc to use. This lives in `/.picoclaw/config.json` by default but using (`PICOCLAW_CONFIG=./config/config.json`) makes it alot easier 

^ make sure this is implemented

- `workspace/` is the placeholder sandbox. PicoClaw reads and writes heres maintaining boundaries (v important)
- `scripts/` is the personal customization of modular functionality like (auto read the news for today from x summarize and return to me) with so much potential
- `.env` holds api keys (dont commit) its included in the .gitignore



## Additional-Info

This portion is for logging or storing notes relevent to the project and its scope. I want to include down the line a file that shuffles an img's data (.txt) by some key. to start simple I want to attempt practical use of key , i.e. you can select a key (.txt) file to both apply and decode a shuffle , such that the key can be any length (0-somearbitrarynumber). any key of any size should function being applied to any encoded file of any size.

_future implementations_
- key ? (of multiple types {.txt , hash / sha256})
- 
# PicoClaw (v1)

This is a repository containing PicoClaw , an ultra lightweight high performance agentic framework. A pseudo operating system for agents

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Project-Structure](#Project-Structure)
- [Usage](#usage)
- [Configuration](#Configuration)
- [Additional-Information](#Additional-Info)

## Introduction

PicoClaw itself is a standalone open source Go program (resource friendly) , acts as a central brain that connects chosen model via api key to real world with examples like (communication via messaging platforms, execute commands tools and skills on laptop, remember storing convo history in local SQLite db or writing to .md)

## Features

- Ultra low RAM usage ; an old ass laptop with limited RAM is more than enough
- Blazing fast boot ; ready almost instantly
- Single binary deployment ; Download 1 file, make it executable and run it
- Built in agent system ; allows for personalization of unique agents for different tasks
- Integrated tools ; bring foundationals and essentails to build up on or off of
- Built in memory ; uses SQLite to remember conversations & context

## Project-Structure

PicoClaw-v1/
- `PicoClaw` (main executable binary)
- config/
    - `config.json` (core PicoClaw configuration)
    - agents/ (optional custom agent definitions)
        - `specific-role.json`
    - gateways/ (optional separate config files for each chat platform)
        - `telegram.json`
        - `discord.json`
        - `whatsapp.json`
    - skills/ (optional user installed skills/plugins for PicoClaw)
        - custom-skill/
- workspace/
    - agent-sessions/ (isolated folders for each agent's work)
        - default/
        - specific-agent/
- scripts/ (utility scripts for automation)
    - `backup-agent-logs.sh`
    - `health-check.sh`
    - `setup-ollama.sh`
- logs/
    - `picoclaw.log`
    - `messaging-platform-gateway.log`
- docs/
    - `ReadMe.md`
    - `other.md`
- `.env` (storing sensitive data , api key)

## Usage 

1. git clone & cd in

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
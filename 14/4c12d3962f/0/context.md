# Session Context

## User Prompts

### Prompt 1

Is there a Claude skill that can merge the pull request and resolve the pull request conflict if there is any, or, if there are any code review commits, can it resolve it before it closes it automatically? Right now I have to keep telling the Claude agent to check this particular PR, and I'm not able to do it in bulk like I would want to.

### Prompt 2

Oh, so does a schedule skill not get triggered while I am not on my laptop, or when I am not actively using Claude?

### Prompt 3

# Schedule Remote Agents

You are helping the user schedule, update, list, or run **remote** Claude Code agents. These are NOT local cron jobs — each trigger spawns a fully isolated remote session (CCR) in Anthropic's cloud infrastructure on a cron schedule. The agent runs in a sandboxed environment with its own git checkout, tools, and optional MCP connections.

## First Step

Your FIRST action must be a single AskUserQuestion tool call (no preamble). Use this EXACT string for the `question` ...

### Prompt 4

Yeah, do you think that makes sense? Is that what people do when they have a lot of pull requests to close, which keep getting added every hour or two?

### Prompt 5

yes

### Prompt 6

yes

### Prompt 7

yes please

### Prompt 8

Base directory for this skill: /Users/jobinlawrance/.claude/plugins/cache/claude-plugins-official/superpowers/5.0.5/skills/brainstorming

# Brainstorming Ideas Into Designs

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design and get user approval.

<HARD-GATE>
Do NOT invoke any implementatio...

### Prompt 9

yes

### Prompt 10

yes

### Prompt 11

both

### Prompt 12

c

### Prompt 13

yup

### Prompt 14

yes

### Prompt 15

yes

### Prompt 16

yes asap

### Prompt 17

start

### Prompt 18

Base directory for this skill: /Users/jobinlawrance/.claude/plugins/cache/claude-plugins-official/superpowers/5.0.5/skills/writing-plans

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it. Give them the whole plan as bite-sized tasks. DRY. YAGNI. TDD. Frequent co...


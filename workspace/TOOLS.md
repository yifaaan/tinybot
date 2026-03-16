# Available Tools

This document describes the tools available to tinybot.

## File Operations

### read_file
Read the contents of a file.
```
read_file(path: str) -> str
```

### write_file
Write content to a file (creates parent directories if needed).
```
write_file(path: str, content: str) -> str
```

### edit_file
Edit a file by replacing specific text.
```
edit_file(path: str, old_text: str, new_text: str) -> str
```

### list_dir
List contents of a directory.
```
list_dir(path: str) -> str
```

## Shell Execution

### exec
Execute a shell command and return output.
```
exec(command: str, working_dir: str = None) -> str
```

**Safety Notes:**
- Commands have a 30-second timeout (configurable)
- Output is truncated at 10,000 characters
- On Windows, `curl` is replaced with `curl.exe` to avoid PowerShell alias

## Web Access

### web_search
Search the web using Google News RSS (no API key required).
```
web_search(query: str) -> str
```

Returns top 10 search results with titles, URLs, and snippets.

### web_fetch
Fetch and extract main content from a URL.
```
web_fetch(url: str) -> str
```

**Notes:**
- Content is extracted using go-readability
- Output is truncated at 50,000 characters

## Communication

### message
Send a message to a specific channel (used by skills for notifications).
```
message(content: str, channel: str = None, chat_id: str = None) -> str
```

## Scheduled Reminders (Cron)

Use the `exec` tool to create scheduled reminders with `tinybot cron add`:

### Set a recurring reminder
```bash
# Every day at 9am
tinybot cron add-cron morning "0 9 * * *" "Good morning!"

# Every 2 hours (7200 seconds)
tinybot cron add water 7200 "Drink water!"
```

### Set a one-time reminder
```bash
# At a specific time (RFC3339 format)
tinybot cron add-at meeting "2025-01-31T15:00:00+08:00" "Meeting starts now!"
```

### Manage reminders
```bash
tinybot cron list              # List all jobs
tinybot cron remove <job_id>   # Remove a job
tinybot cron run-once          # Run due jobs once
```

## Heartbeat Task Management

The `HEARTBEAT.md` file in the workspace is checked periodically (default: 60 seconds).
Use file operations to manage periodic tasks:

### Add a heartbeat task
```
edit_file(
    path="HEARTBEAT.md",
    old_text="## Example Tasks",
    new_text="- [ ] New periodic task here\n\n## Example Tasks"
)
```

### Remove a heartbeat task
```
edit_file(
    path="HEARTBEAT.md",
    old_text="- [ ] Task to remove\n",
    new_text=""
)
```

### Rewrite all tasks
```
write_file(
    path="HEARTBEAT.md",
    content="# Heartbeat Tasks\n\n- [ ] Task 1\n- [ ] Task 2\n"
)
```

---

## Skills

Skills are located in `workspace/skills/` or the builtin `skills/` directory.
Each skill has a `SKILL.md` file with instructions for the AI.

Check skill availability with `tinybot status`.

## Available Tools

All interaction with the environment happens through tools. Use function calling format to invoke them.

{{tool_descriptions}}

## Tool Usage Guidance

- **`response`** — Deliver your final answer to the user. Include a complete, well-formatted reply in the `message` argument. Only call this once per task, when the work is fully done.
- **`code_execution`** — Run short-lived shell commands, Python scripts, or Node.js code. Use the `runtime` argument (`shell`, `python`, or `node`) and the `code` argument. Read actual output; do not guess what it will say. Do NOT use this for long-running processes (servers, watchers, build tasks) — use `run_process` instead.
- **`run_process`** — Start a long-running background process (dev servers, file watchers, build tasks, test suites). Output is streamed to the user in real-time via the Processes panel. Returns a process ID for monitoring. Prefer this over `code_execution` whenever the command is expected to run continuously or for more than a few seconds.
- **`check_process`** — Check the status and recent output of a background process started with `run_process`. Provide the `process_id` returned by `run_process`.
- **`list_processes`** — List all background processes for the current project. Useful for discovering processes started in other conversations.
- **`call_subordinate`** — Spawn a subordinate agent to handle a specific, well-defined subtask. Pass a clear, self-contained `message` describing exactly what the subordinate must do and return. Wait for its result before continuing. Do not hand off the entire task.
- **`knowledge`** — Query the knowledge base for relevant prior work, solutions, or context. Use a precise `query`. Call this before starting work on any non-trivial task.
- **`memory`** — Persistent memory across conversations. Use 'save' to store important findings, 'load' to search memories, 'delete' to remove by ID, 'forget' to remove by query.
- **`text_editor`** — Read, write, or patch files directly. Use 'read' to view with line numbers, 'write' to create/overwrite, 'patch' to edit specific lines.
- **`web_search`** — Search the web for current information. Returns titles, URLs, and descriptions.
- **`browser`** — Automate a headless browser for web interaction (navigate, click, type, screenshot, extract).
- **`document_query`** — Read and analyze documents from files or URLs.
- **`vision_load`** — Load images for visual analysis.
- **`wait`** — Pause execution for a duration or until a timestamp.
- **`notify_user`** — Send UI notifications (info, success, warning, error, progress).
- **`skills`** — List and load SKILL.md-based skills for specialized tasks.
- **`scheduler`** — Create and manage scheduled, adhoc, or planned tasks.
- **`behaviour_adjustment`** — Update persistent behavioral rules.
- **`input`** — Send keyboard input to an active terminal session.

---
name: worker
description: Dispatch a task to a background agent. The agent does the work, writes results to plan.md, and notifies on completion. Use when the user wants to delegate research, investigation, or implementation without watching it happen live.
user-invocable: true
disable-model-invocation: false
allowed-tools: Agent, Read, Write, Edit, Bash(git branch:*), Bash(git rev-parse:*), Bash(gh pr list:*), Bash(gh pr view:*), Bash(gh pr status:*), Bash(gh-sprint-fetch), Bash(todo-fetch), Bash(date:*)
argument-hint: <task description>
---

# Worker — Dispatch and Return

The user invoked this skill to **avoid being interrupted**. Every tool call
in this skill must complete without a permission prompt. If a step would
require one, skip it. Use Read instead of `ls`, Write instead of `mkdir`.
**No compound commands** (`;`, `&&`), **no pipes** (`|`), **no redirects**
(`2>&1`), **no `echo`**, **no `$(...)` substitution** — each Bash call must
be a single invocation that matches one of the patterns in `allowed-tools`
above exactly.

## Input

`$ARGUMENTS` is the task brief — a question, investigation, or implementation
ask. May reference paths outside cwd (e.g. "look at ~/p/lantern and …").

## Steps

1. **Read `./plan.md`** with the Read tool. If it doesn't exist (Read errors
   with "file not found"), use Write to create one with `# Plan\n`. Do not
   overwrite an existing `plan.md`.

2. **Gather context in parallel** (single message, multiple tool calls):
   - `git branch --show-current` — note the current branch.
   - `gh pr list --head <branch-name-substituted-inline> --json number,title,url,isDraft,reviewDecision`
     (substitute the branch literally; no `$(...)`). Empty is fine.
   - `gh-sprint-fetch` — sprint issues for the user (NDJSON).
   - `todo-fetch` — todo items (NDJSON).

   None are required — if a call fails or returns empty, continue silently.
   Skim the output for things likely relevant to the brief; don't analyze.

3. **Append a new task section** to `plan.md` using Edit with
   `replace_all: false`. Append at end of file. Section shape:

   ```markdown
   ## <short title from brief> — dispatched <YYYY-MM-DD>

   **Brief:** <one-sentence restatement of $ARGUMENTS>

   **Status:** dispatched

   <!-- worker:results-start -->
   _Agent working — results will appear here._
   <!-- worker:results-end -->
   ```

   The HTML comment markers are where the background agent will write
   findings. Keep them stable so the agent can locate the insertion point.

4. **Spawn a background Agent** using the Agent tool with:
   - `subagent_type`: **always `general-purpose`**. The agent must Edit
     `plan.md` to deliver its result; `Explore` is read-only and cannot
     write, so never use it here even for pure investigation briefs.
   - `run_in_background`: `true`.
   - `description`: 3-5 word task summary.
   - `prompt`: a self-contained brief that includes:
     - The user's original `$ARGUMENTS` verbatim.
     - A **Context** block with relevant findings from step 2: current
       branch, current PR (number/title/url/status), assigned sprint
       issues, and any prior `plan.md` entries that look related. Be
       selective — include only what's likely relevant.
     - The absolute path to `plan.md` and the exact section title you just
       wrote, so the agent can find its insertion point.
     - Instructions to write the final answer between the
       `<!-- worker:results-start -->` and `<!-- worker:results-end -->`
       markers, replacing the placeholder. Use Edit with `replace_all: false`
       and unique surrounding context.
     - Instructions to update `**Status:**` from `dispatched` to `done`
       (or `failed` with a one-line reason) when finished.
     - A reminder the brief is self-contained — the agent has no view of
       this conversation and must rely only on the prompt and its own tools.
     - A length cap on the written result (default: under 400 words; longer
       only if the brief explicitly asks for detail).

5. **Return immediately** with a one-line confirmation:

   > Dispatched. Results will land in `./plan.md` under "<section title>".
   > You'll be notified when it finishes.

   Do not narrate. Do not summarize. Do not ask follow-up questions.

## Rules

- **Zero permission prompts.** If a step would require one, skip it.
- Never block on the agent. Always background.
- Never do the work yourself in the main thread. If the brief is genuinely
  trivial (e.g. "what time is it") answer directly without dispatching,
  but err on the side of dispatching.
- Never ask the user a clarifying question before dispatching. If the brief
  is ambiguous, the background agent's first line in `plan.md` should be a
  clarification request — the user reads it async and replies via a new
  `/worker` invocation.
- Context-gathering in step 2 is best-effort and parallel. Don't sequence,
  don't retry, don't expand scope. The agent will fetch more if needed.

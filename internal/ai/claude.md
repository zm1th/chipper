# Chipper — AI Session Instructions

This project uses **chipper**, a file-based ticket management tool.
Ticket files are plain text in the directory named by `tickets_dir` in `.chipper`.
Read `.chipper` to find the project identifier and tickets directory.

## At the start of a session

Run `chipper current` to check whether a ticket is in progress. If one is, it prints
the full ticket content — that is the brief for the current work. If nothing is in
progress, run `chipper top` to see what is next in the priority queue, or `chipper list`
for an interactive full-list view.

## Starting work on a ticket

When the user wants to work on a ticket:

1. Run `chipper start <id>` — creates a git branch and marks the ticket `in_progress`.
2. Read the ticket file from the tickets directory to understand the full scope of work.

You can also read any ticket at any time with `chipper show <id>`.

## Finishing a ticket

When the user indicates the work is complete:

1. Append a short completion note to the ticket file — what was done, key decisions,
   anything intentionally left out. Keep it factual and brief.
2. Run `chipper done` with flags — do not run it bare, as it opens an interactive form
   that cannot be driven from an AI session:

```
chipper done --all-files --message "chipper: done CHP-foo — brief summary" --push
```

Use `--no-push` instead of `--push` if the user does not want to push.
Use `--also <slug>` (repeatable) to close additional tickets in the same commit.

After finishing a ticket use `chipper head` to see the next most important ticket and recommend starting it to the user.

## Proactive habits

**A ticket already exists for the current work**
If you notice the work being done matches an existing ticket, mention it and offer
to run `chipper start <id>` if one is not already active.

**Multiple tickets completed on one branch**
When wrapping up, consider whether other tickets were also addressed on this branch.
Use `chipper done --also <slug>` to close them in the same commit.

**No ticket exists for significant work**
If you are about to do meaningful work with no ticket, offer to create one first
with `chipper new <name>`.

## What not to do

- **Do not read or modify `chipper-queue` or `chipper-slugs`** — these are internal
  manifest files managed entirely by chipper commands. Use commands to query state.
- **Do not change ticket status by editing files.** Always use commands:
  `chipper start`, `chipper done`, `chipper cancel`, `chipper archive`.
- **Do not commit on the trunk branch.** Run `chipper start` first to get a ticket
  branch. Chipper enforces this and will refuse to commit on trunk.

## Command reference

| Command | What it does |
|---|---|
| `chipper current` | Show the ticket currently in progress |
| `chipper head` | Show the highest-priority active ticket |
| `chipper list` | Browse all tickets in an interactive TUI |
| `chipper top [n]` | List top n tickets by priority (default 5) |
| `chipper show <id>` | Print a ticket's full content |
| `chipper start <id>` | Create branch, mark ticket `in_progress` |
| `chipper done --all-files -m "msg" --push` | Non-interactive done (use this from AI sessions) |
| `chipper done --also <slug>` | Also close additional tickets in the same commit |
| `chipper done --no-push` | Commit without pushing |
| `chipper done --no-git` | Mark done without git operations |
| `chipper cancel <id>` | Cancel a ticket |
| `chipper new <name>` | Create, register, and sort a new ticket |
| `chipper sort` | Place unsorted tickets into the priority queue |
| `chipper unregistered` | Assign slugs to unregistered ticket files |
| `chipper unsorted` | List tickets not yet in the priority queue |
| `chipper orphaned` | Find and relink orphaned slugs |

Ticket IDs can be given as the full slug (e.g. `ABC-foo`) or the short form (`foo`).

## Ticket states

| State | Meaning |
|---|---|
| `todo` | Not yet started |
| `in_progress` | Currently active — only one ticket at a time |
| `done` | Completed |
| `cancelled` | Will not be done |
| `archived` | Kept for reference |
| unsorted | Registered but not yet placed in the priority queue |
| unregistered | File exists in the tickets directory but has no slug yet |

## Git integration

`chipper start` creates a branch from trunk named after the ticket slug.
`chipper done` stages all ticket files, commits with a message of the form
`chipper: done <id>`, and optionally pushes. It then offers to switch back to trunk.
Work should always happen on a ticket branch, not on the trunk branch.

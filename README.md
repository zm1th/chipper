# Chipper

Chipper is a CLI tool for managing tickets as plain text files within your project repository. Each unit of work ("chip") lives as a committed file, keeping your ticket history alongside your code.

## Philosophy

- Tickets are files. Create one by creating a file.
- Files never move or get renamed to indicate status — a single manifest tracks everything.
- The ticket queue is always sorted by priority.
- Commands are short and AI-friendly: easy to invoke from a Claude session or your terminal.

## Installation

**Homebrew (macOS):**
```sh
brew install <tap>/chipper
```

**From source (requires Go):**
```sh
git clone https://github.com/zm1th/chipper
cd chipper
make install
```

## Getting Started

```sh
chipper init
```

This prompts for your project identifier and tickets directory, then creates the `.chipper` config file and the tickets directory with its manifest files. Any value can be skipped at the prompt by passing it as a flag instead:

```sh
chipper init --project CHP --dir path/to/my-tickets
```

## Ticket Files

Tickets are files inside your tickets directory. They can be any text-based format — plain text, Markdown, or anything else readable as text. Filenames have no required extension, and different tickets in the same project can use different formats. Chipper treats them all the same; the content is yours to structure however you like.

Binary formats (PDF, DOCX, images) are not recommended. They bloat the repository, don't diff meaningfully, and are harder for AI tools to read. If a ticket needs to reference a mockup, screenshot, or other asset, store the file separately and link to it by path or URL from within the ticket text.

You can create a ticket either way:

```sh
# Just create a file directly
touch chipper-tickets/add-login-page

# Or let chipper create and register it in one step
chipper new add-login-page
```

Both are fully supported. If you create files manually, run `chipper register` to assign them slugs, or `chipper list --unregistered` to see what's waiting. New tickets are unsorted until you run `chipper sort`.

## Project Identifier and Slugs

Each project has a short identifier set in `.chipper` (e.g. `CHP`). Each ticket gets a short, unique slug (e.g. `initial`). Together they form a ticket ID:

```
CHP-initial
```

Slugs are easier to type than numbers and work well in git branch names (e.g. `feat/CHP-initial`).

Slugs are assigned in the manifest. Chipper will suggest a slug based on the filename, but you can override it. Slugs must be unique within a project — chipper will reject or prompt for a different slug if there is a conflict.

## Commands

| Command | Description |
|---|---|
| `chipper init` | Initialize chipper interactively, prompting for project identifier and tickets directory; flags override prompts |
| `chipper new <name>` | Create a new ticket file and register it; opens `$EDITOR` if set, otherwise accepts multi-line input terminated by Ctrl+D; once content is saved, runs the interactive sort to place it in the priority queue |
| `chipper register` | Scan the tickets directory and register any unregistered files |
| `chipper sort` | Interactively sort unsorted tickets into the priority queue |
| `chipper list` | List all tickets with their status and priority order |
| `chipper top [n]` / `chipper list --top [n]` | List the slugs and filenames of the top n active (not completed) tickets by priority; defaults to 5 if n is omitted |
| `chipper head` | Display the full content of the highest-priority active ticket |
| `chipper unsorted` / `chipper list --unsorted` | List tickets not yet placed in the priority queue |
| `chipper list --unregistered` | List ticket files not yet assigned a slug |
| `chipper unregistered` | List unregistered files and interactively prompt to assign a slug to each one |
| `chipper show <id>` | Display a ticket's content |
| `chipper start <id>` | Mark a ticket in progress, commit the manifest update, and create+checkout a git branch named after the ticket |
| `chipper done` | Mark the in-progress ticket complete; interactively prompts for any other tickets also finished on this branch; `--also slug1,slug2` skips the prompt |
| `chipper cancel <id>` | Mark a ticket cancelled and commit the manifest update |
| `chipper archive <id>` | Archive a ticket (remove from active queue, keep the file) and commit the manifest update |
| `chipper slug <id> <new-slug>` | Rename a ticket's slug; interactively offers to update all references to the old ID across ticket files |
| `chipper mv <id> <new-filename>` | Rename a ticket file on disk and update the manifest |
| `chipper relink <id> <filename>` | Link an orphaned slug to an existing file (e.g. after a manual rename) |
| `chipper list --orphaned` | List slugs whose ticket files no longer exist on disk |
| `chipper orphaned` | List orphaned slugs and interactively prompt to relink each one to a new filename |
| `chipper priority <id> <index>` | Set a specific sort index for a ticket; if the index is taken, the existing ticket is bumped to the next available index below it |
| `chipper ai` | Interactively generate AI instruction files for supported coding tools |
| `chipper <id> depends-on <id>` | Declare that a ticket depends on another |
| `chipper <id> remove-dep <id>` | Remove a dependency relationship |
| `chipper deps <id>` | Show all dependencies for a ticket |
| `chipper blocking <id>` | Show all tickets that depend on the given ticket |

### Examples

```sh
chipper start CHP-initial
chipper done CHP-initial
chipper list
chipper show CHP-login
```

`chipper start` will warn you if another ticket is already in progress.

## Priority Sorting

Chipper keeps the active queue sorted at all times. When you add new tickets, they are unsorted. Run `chipper sort` to place them.

For each unsorted ticket, chipper uses binary search to find its position:

1. You are shown the new ticket title alongside the title of the queue's midpoint ticket.
2. You choose which is higher priority.
3. Chipper recurses into the appropriate half of the list.
4. Once the candidate segment is small (fewer than 10 tickets), you are shown the full segment and can either continue with midpoint comparisons or pick an exact insertion index directly.

This requires at most log₂(queue size) comparisons per ticket.

## The Manifest

Two files live inside your tickets directory and together form the manifest.

**`chipper-slugs`** — maps filenames to slugs, one entry per line:

```
initial-chipper-project-ideas = initial
add-login-page = login
dark-mode = dark-mode
```

**`chipper-queue`** — tracks status and optional priority index per slug, written in three sections:

```
login            = in_progress  2000
dark-mode        = todo         3000

new-idea         = todo

initial          = done
old-task         = cancelled
```

- **Top section**: active tickets with a priority index, sorted ascending
- **Middle section**: active tickets not yet sorted into the priority queue
- **Bottom section**: completed tickets (done, cancelled, archived), appended over time

Completed tickets lose their priority index — they are never re-sorted and simply accumulate at the bottom as a record of finished work.

Gaps between indexes (default: 1000) mean that inserting or repositioning a ticket rarely causes conflicts. When conflicts do occur, only the affected entries are renumbered — not the rest of the list.

**`chipper-dependencies`** (optional) — declares relationships between tickets:

```
login depends-on scaffold
dashboard depends-on login
```

When present, chipper can warn if a ticket is started before its dependencies are complete, and can factor dependencies into sort suggestions. This file is optional and not required for basic chipper usage.

Valid statuses: `todo`, `in_progress`, `done`, `cancelled`, `archived`

Ticket states:

| State | Meaning |
|---|---|
| **unregistered** | File exists but has no slug in `chipper-slugs` |
| **unsorted** | Has a slug but no entry in `chipper-queue` |
| **orphaned** | Has a slug or queue entry but the file no longer exists on disk |

All three are surfaced by chipper rather than treated as errors. Orphaned tickets most commonly result from a file being renamed directly on disk. When unregistered files are found alongside orphaned entries, chipper will offer to link them — treating the unregistered file as a rename of the orphan and preserving its slug, status, and priority.

## Git Integration

Chipper runs git commands automatically as part of the ticket lifecycle when `git = true` is set in `.chipper` (the default when initializing inside a git repository).

| Lifecycle event | Git action |
|---|---|
| `chipper start` | Commits the manifest update, creates and checks out a branch named `<project>-<slug>` |
| `chipper done` | Opens an interactive staging interface to review and commit changes, then prompts to push |

Branch names follow the pattern `CHP-login` by default. A prefix can be configured (e.g. `branch_prefix = feat` yields `feat/CHP-login`).

Chipper will never commit changes while on the trunk branch. All commits happen on a ticket branch created by `chipper start`.

Git integration can be disabled per-command with `--no-git`, or globally by setting `git = false` in `.chipper`.

## Configuration

`.chipper` is a simple key-value config file at your project root:

```
project = CHP
tickets_dir = chipper-tickets
default_status = todo
git = true
branch_prefix =
```

To store tickets outside the project (e.g. to omit them from deployments), point `tickets_dir` at an absolute path or a path outside the project tree.

## AI Usage

Chipper is designed to work naturally inside AI coding sessions. You can say things like:

- "start the login ticket" → `chipper start CHP-login`
- "what's in progress?" → `chipper list`
- "mark initial done" → `chipper done CHP-initial`

Chipper can also append AI-generated summaries to ticket files — notes on decisions made, context behind the work, and actions taken — keeping a useful record alongside the original ticket description.

### AI Instruction Files

Chipper can generate instruction files for popular AI coding tools so they understand chipper commands and conventions without any extra explanation from you:

```sh
chipper ai
```

This interactively prompts you to select which tools to generate files for. You can also target a specific tool directly:

```sh
chipper ai --claude     # CLAUDE.md
chipper ai --cursor     # .cursor/rules/chipper.mdc
chipper ai --copilot    # .github/copilot-instructions.md
chipper ai --windsurf   # .windsurfrules
```

Generated files explain the manifest format, available commands, ticket states, and slug conventions. Re-run `chipper ai` after upgrading chipper to refresh them.

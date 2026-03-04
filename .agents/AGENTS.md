# Agent Instructions

- AGENTS.md and skills canonically live in `~/.agents` globally and `<project>/.agents` locally.
- Any time AGENTS.md or skills are created/updated globally or locally, ensure symlinks exist from `.claude` to canonical paths:
- `~/.claude/CLAUDE.md` -> `~/.agents/AGENTS.md`
- `<project>/.claude/CLAUDE.md` -> `<project>/.agents/AGENTS.md`
- `~/.claude/skills` -> `~/.agents/skills`
- `<project>/.claude/skills` -> `<project>/.agents/skills`

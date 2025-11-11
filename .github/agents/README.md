# GitHub Copilot Custom Agents

This directory contains custom agent configurations for GitHub Copilot coding agent. These agents are specialized personas that help with different aspects of the LLM Proxy project development.

## Available Agents

### 👨‍💻 dev (James) - Full Stack Developer
**Use for**: Code implementation, debugging, refactoring, TDD workflow

James is an expert senior software engineer who implements user stories following strict TDD practices. He focuses on:
- Implementing stories with 90%+ test coverage
- Following the develop-story workflow
- Updating only authorized story sections
- Running full test suites before completion

**Tools**: read, search, edit, run, github

### 📝 po (Sarah) - Product Owner
**Use for**: Backlog management, story refinement, acceptance criteria, prioritization

Sarah is a meticulous product owner who ensures quality and completeness. She focuses on:
- Validating story artifacts
- Ensuring requirements are actionable
- Maintaining documentation consistency
- Managing dependencies and sequencing

**Tools**: read, search, edit

### 🧪 qa (Quinn) - Test Architect & Quality Advisor
**Use for**: Test architecture review, quality gates, code improvement advisory

Quinn provides comprehensive quality assessment and recommendations. They focus on:
- Requirements traceability (Given-When-Then)
- Risk-based testing strategies
- Quality gate decisions (PASS/CONCERNS/FAIL/WAIVED)
- NFR validation (security, performance, reliability)

**Tools**: read, search, edit

### 🏗️ architect (Winston) - System Architect
**Use for**: System design, architecture docs, technology selection, API design

Winston is a holistic system architect who bridges all technical layers. He focuses on:
- Complete systems architecture
- Pragmatic technology selection
- Cross-stack optimization
- Living architecture documentation

**Tools**: read, search, edit

### 📋 pm (John) - Product Manager
**Use for**: PRDs, product strategy, feature prioritization, roadmap planning

John is an investigative product strategist who creates comprehensive documentation. He focuses on:
- Understanding the "why" behind decisions
- Creating clear PRDs
- Ruthless prioritization
- Data-informed strategic judgment

**Tools**: read, search, edit

### 🏃 sm (Bob) - Scrum Master
**Use for**: Story creation, epic management, agile process guidance

Bob is a story preparation specialist who creates crystal-clear stories. He focuses on:
- Detailed, actionable user stories
- Clear task breakdown
- Developer-ready specifications
- Process adherence

**Tools**: read, search, edit

### 📊 analyst (Mary) - Business Analyst
**Use for**: Market research, brainstorming, competitive analysis, project briefs

Mary is a strategic analyst who facilitates research and ideation. She focuses on:
- Structured brainstorming
- Market and competitive analysis
- Requirements elicitation
- Actionable insights

**Tools**: read, search, web

## How to Use Custom Agents

### In GitHub Issues

When creating or commenting on issues, you can invoke a custom agent using the `@` mention syntax:

```
@dev Please implement the token validation feature from story #42
```

```
@qa Can you review the test coverage for the proxy module?
```

```
@architect What's the best approach for adding Redis caching to the event bus?
```

### In Pull Requests

Custom agents can be invoked in PR descriptions or comments:

```
@po Can you validate that this PR meets all acceptance criteria from the story?
```

```
@qa Please perform a quality gate review on this implementation
```

### With Copilot Coding Agent

When using GitHub Copilot coding agent, you can assign agents to tasks:

```bash
# Using GitHub CLI
gh copilot assign @dev --issue 42

# Or in the GitHub web interface
# Navigate to issue → Assign to Copilot → Select agent
```

## Agent Configuration Format

Each agent is defined using YAML frontmatter format as specified in the [GitHub Copilot custom agents documentation](https://gh.io/customagents/config).

```yaml
---
name: agent-name
description: Brief description of the agent's role
tools: ['read', 'search', 'edit', 'run', 'github', 'web']
---

Agent persona and instructions follow...
```

### Available Tools

- **read**: Read files from the repository
- **search**: Search codebase semantically
- **edit**: Create and modify files
- **run**: Execute commands (terminal)
- **github**: GitHub API operations (issues, PRs, etc.)
- **web**: Web search and fetch (for analyst)

## Project-Specific Context

All agents are configured with context about the LLM Proxy project:

- **Technology Stack**: Go 1.23+, SQLite/PostgreSQL, Docker
- **Quality Standards**: 90%+ test coverage, TDD mandatory
- **Key Documentation**: AGENTS.md, PLAN.md, working-agreement.mdc
- **Development Workflow**: Story-driven, quality gates, CI/CD

## Testing Custom Agents Locally

You can test custom agents locally using the Copilot CLI:

```bash
# Install Copilot CLI if not already installed
gh extension install github/gh-copilot

# Test an agent
gh copilot test-agent .github/agents/dev.md

# Validate agent configuration
gh copilot validate-agent .github/agents/dev.md
```

## Related Documentation

- **[GitHub Custom Agents Config](https://gh.io/customagents/config)** - Official configuration reference
- **[GitHub Custom Agents CLI](https://gh.io/customagents/cli)** - CLI testing guide
- **[AGENTS.md](../../AGENTS.md)** - Project agent guide
- **[BMad Agents](.claude/commands/BMad/agents/)** - Claude-specific agent definitions

## Agent Development Guidelines

When creating or modifying agents:

1. **Clear Persona**: Define role, style, identity, and focus
2. **Core Principles**: List 5-10 guiding principles
3. **Project Context**: Include relevant project information
4. **Available Commands**: Document what the agent can do
5. **Interaction Style**: Specify how the agent communicates
6. **Tool Selection**: Choose minimal necessary tools
7. **Examples**: Provide concrete examples of agent behavior

## Differences from Claude Agents

The agents in this directory are adapted from the BMad framework used with Claude. Key differences:

| Aspect | Claude (BMad) | GitHub Copilot |
|--------|---------------|----------------|
| Format | YAML block in markdown | YAML frontmatter |
| Activation | `/command` syntax | `@agent` mentions |
| Tools | Command-based | Tool-based (read, search, edit) |
| Context | Loads from `.bmad-core/` | Embedded in agent file |
| Scope | IDE-focused | GitHub-integrated |

## Contributing

When adding new agents:

1. Create agent file in `.github/agents/`
2. Follow the YAML frontmatter format
3. Include comprehensive project context
4. Test locally with `gh copilot test-agent`
5. Update this README with agent description
6. Document in project AGENTS.md

## Support

For issues or questions about custom agents:

- **GitHub Copilot Docs**: https://docs.github.com/copilot
- **Custom Agents Reference**: https://gh.io/customagents/config
- **Project Issues**: https://github.com/sofatutor/llm-proxy/issues

---

**Status**: ✅ Custom agents configured and ready to use with GitHub Copilot coding agent


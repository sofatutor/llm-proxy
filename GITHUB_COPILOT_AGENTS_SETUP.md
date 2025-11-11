# GitHub Copilot Custom Agents Setup

This document summarizes the GitHub Copilot custom agent configuration for the LLM Proxy project.

## What Was Configured

Custom agents have been created in `.github/agents/` to work with GitHub Copilot coding agent. These agents are specialized personas adapted from the BMad framework for GitHub's environment.

## Available Agents

| Agent | Name | Role | Primary Use Cases |
|-------|------|------|-------------------|
| **dev** | James | Full Stack Developer | Code implementation, debugging, TDD workflow |
| **po** | Sarah | Product Owner | Story refinement, acceptance criteria, backlog management |
| **qa** | Quinn | Test Architect | Quality gates, test architecture, code review |
| **architect** | Winston | System Architect | Architecture docs, system design, tech selection |
| **pm** | John | Product Manager | PRDs, product strategy, feature prioritization |
| **sm** | Bob | Scrum Master | Story creation, epic management, process guidance |
| **analyst** | Mary | Business Analyst | Market research, brainstorming, competitive analysis |

## How to Use

### In GitHub Issues

Mention agents in issue comments:

```
@dev Please implement the token validation feature from story #42
```

```
@qa Can you review the test coverage for the proxy module?
```

```
@architect What's the best approach for adding Redis caching?
```

### In Pull Requests

Invoke agents in PR descriptions or comments:

```
@po Validate that this PR meets all acceptance criteria
```

```
@qa Perform a quality gate review on this implementation
```

### With Copilot Coding Agent

Assign agents to tasks using GitHub CLI or web interface:

```bash
gh copilot assign @dev --issue 42
```

Or in the GitHub web interface:
1. Navigate to an issue
2. Click "Assign to Copilot"
3. Select the appropriate agent

## Agent Configuration

Each agent is configured with:

### 1. YAML Frontmatter
```yaml
---
name: agent-id
description: Brief description of role
tools: ['read', 'search', 'edit', 'run', 'github', 'web']
---
```

### 2. Persona Definition
- **Role**: What they do
- **Style**: How they work
- **Identity**: Who they are
- **Focus**: What they prioritize

### 3. Core Principles
5-10 guiding principles that shape behavior

### 4. Project Context
- Technology stack (Go 1.23+, SQLite/PostgreSQL)
- Quality standards (90%+ coverage, TDD)
- Key documentation references
- Development workflow

### 5. Available Commands
What the agent can help with

### 6. Interaction Style
How the agent communicates

## Tool Permissions

Each agent has specific tools enabled:

| Tool | Purpose | Agents |
|------|---------|--------|
| **read** | Read repository files | All agents |
| **search** | Semantic code search | All agents |
| **edit** | Create/modify files | All agents |
| **run** | Execute terminal commands | dev |
| **github** | GitHub API operations | dev |
| **web** | Web search and fetch | analyst |

## Project-Specific Configuration

All agents are configured with LLM Proxy project context:

### Technology Stack
- Go 1.23+
- SQLite (dev) / PostgreSQL (prod)
- Docker deployment
- Event-driven architecture

### Quality Standards
- **Test Coverage**: 90%+ enforced
- **TDD**: Mandatory failing test first
- **Validation**: `make test`, `make lint`, `make test-race`
- **No Merges**: Without passing tests and linters

### Key References
- **AGENTS.md**: Primary agent guide
- **PLAN.md**: Project architecture and objectives
- **working-agreement.mdc**: Core development workflow
- **docs/README.md**: Complete documentation index

### Development Workflow
1. Stories created from PRDs
2. TDD: Test first, implement, validate
3. Quality gates before merge
4. 90%+ coverage required
5. All linters must pass

## Testing Agents Locally

Use the GitHub Copilot CLI to test agents:

```bash
# Install Copilot CLI extension
gh extension install github/gh-copilot

# Test an agent
gh copilot test-agent .github/agents/dev.md

# Validate agent configuration
gh copilot validate-agent .github/agents/dev.md

# List all agents
ls -la .github/agents/
```

## Differences from Claude (BMad) Agents

| Aspect | Claude (BMad) | GitHub Copilot |
|--------|---------------|----------------|
| **Location** | `.claude/commands/BMad/agents/` | `.github/agents/` |
| **Format** | YAML block in markdown | YAML frontmatter |
| **Activation** | `/command` syntax | `@agent` mentions |
| **Tools** | Command-based dependencies | Tool permissions (read, search, edit) |
| **Context** | Loads from `.bmad-core/` | Embedded in agent file |
| **Scope** | IDE/Cursor-focused | GitHub-integrated |
| **Execution** | Local IDE | GitHub Copilot service |

## Agent Behavior Examples

### dev (James) - Developer
```
User: @dev Implement token expiration checking

Agent Response:
I'll implement token expiration following TDD workflow:

1. Write failing test in internal/token/validator_test.go
2. Implement IsExpired() method in internal/token/validator.go
3. Run tests and ensure 90%+ coverage
4. Update story file Dev Agent Record section
5. Validate with make test && make lint

Starting with the test...
```

### qa (Quinn) - Quality Advisor
```
User: @qa Review PR #45 for quality gate

Agent Response:
Conducting comprehensive quality review...

**Requirements Traceability**: ✅ PASS
- All acceptance criteria mapped to tests
- Given-When-Then patterns used

**Test Coverage**: ⚠️ CONCERNS
- Achieved: 87% (target: 90%+)
- Missing: Error path in ValidateToken()

**Gate Decision**: CONCERNS
Can proceed with minor fix needed.

Recommendation: Add test case for invalid token format.
```

### architect (Winston) - Architect
```
User: @architect Should we add Redis caching to the event bus?

Agent Response:
Let me analyze this holistically...

**Current State**: In-memory event bus with buffered channels

**Proposed**: Redis-backed event bus for persistence

**Trade-offs**:
✅ Pros:
- Event persistence across restarts
- Multi-instance support
- Replay capability

❌ Cons:
- Added operational complexity (Redis dependency)
- Network latency vs in-memory
- Cost (Redis hosting)

**Recommendation**: 
Start with in-memory for MVP. Add Redis as optional backend when:
1. Multi-instance deployment needed
2. Event persistence required
3. Replay functionality requested

**Implementation Path**:
- Abstract event bus interface
- Keep in-memory as default
- Add Redis adapter as plugin
```

## Integration with Existing Workflow

### Story-Driven Development

1. **@pm** creates PRD
2. **@sm** breaks down into stories
3. **@po** refines and validates stories
4. **@dev** implements with TDD
5. **@qa** performs quality gate review
6. **@architect** reviews architectural decisions

### Quality Gates

1. **Story Ready**: @po validates completeness
2. **Implementation**: @dev follows TDD
3. **Code Review**: @qa performs gate review
4. **Architecture**: @architect validates design
5. **Merge**: All gates pass

## File Structure

```
.github/agents/
├── README.md           → This documentation
├── dev.md             → Developer agent (James)
├── po.md              → Product Owner agent (Sarah)
├── qa.md              → Quality Advisor agent (Quinn)
├── architect.md       → System Architect agent (Winston)
├── pm.md              → Product Manager agent (John)
├── sm.md              → Scrum Master agent (Bob)
└── analyst.md         → Business Analyst agent (Mary)
```

## Resources

- **[GitHub Custom Agents Documentation](https://gh.io/customagents/config)** - Official configuration reference
- **[GitHub Copilot CLI Guide](https://gh.io/customagents/cli)** - CLI testing and validation
- **[.github/agents/README.md](.github/agents/README.md)** - Detailed agent documentation
- **[AGENTS.md](AGENTS.md)** - Project agent guide

## Next Steps

1. **Test Locally**: Use `gh copilot test-agent` to validate configurations
2. **Try in Issues**: Mention agents in GitHub issues to see them in action
3. **Refine**: Adjust agent personas based on actual usage
4. **Expand**: Add more specialized agents as needed
5. **Document**: Update this guide with learnings and best practices

## Troubleshooting

### Agent Not Responding

1. Check agent file syntax: `gh copilot validate-agent .github/agents/dev.md`
2. Verify file is in `.github/agents/` directory
3. Ensure YAML frontmatter is properly formatted
4. Check that agent name matches file name (without .md)

### Agent Behavior Unexpected

1. Review agent persona and core principles
2. Check tool permissions in YAML frontmatter
3. Verify project context is accurate
4. Test with simpler prompts first

### Permission Errors

1. Verify tools in YAML frontmatter
2. Check GitHub Copilot settings
3. Ensure repository has Copilot enabled
4. Verify user has appropriate access

## Support

For issues or questions:

- **GitHub Copilot Support**: https://support.github.com/
- **Project Issues**: https://github.com/sofatutor/llm-proxy/issues
- **Documentation**: [.github/agents/README.md](.github/agents/README.md)

---

**Status**: ✅ GitHub Copilot custom agents configured and ready to use


# Documentation Guidelines

Guidelines for documenting components in the API Platform.

## Constitution

Each component SHOULD have a `spec/constitution.md` that defines core principles and standards for that component.

**Reference Example**: See [platform-api/spec/constitution.md](../platform-api/spec/constitution.md) for Platform API's constitution covering:
- Specification-First Development
- Layered Architecture
- Security by Default
- Documentation Traceability
- RESTful API Standards (including camelCase and list response structure)
- Data Integrity
- Container-First Operations

**When to create a constitution:**
- Component has architectural principles that must be enforced
- Multiple teams contribute to the component
- API standards need to be codified (naming, response structures, error formats)
- Security or compliance requirements exist

**When NOT to create a constitution:**
- Simple single-purpose components
- Components with no external API contracts
- Short-lived experimental projects

## Principles

1. **No unnecessary content** - Keep content concise and to the point
2. **Link requirements to implementation** - Every PRD links to its implementation feature
3. **Single source of truth** - One place for each concept, linked from multiple places
4. **Developer-first** - Quick start should get developers running immediately
5. **Follow the constitution** - Adhere to component-specific standards defined in spec/constitution.md

## Structure

Each component must have this structure:

```
component/
├── README.md                    # Quick start + pointer to spec/
├── spec/
│   ├── README.md                # Spec overview
│   ├── constitution.md          # Core principles and standards (optional)
│   ├── prd.md                   # Product requirements (links to prds/)
│   ├── architecture.md          # Architecture overview
│   ├── design.md                # Design decisions
│   ├── impl.md                  # Implementation overview (links to impls/)
│   ├── prds/                    # Individual product requirements
│   │   ├── feature-1.md
│   │   └── feature-2.md
│   └── impls/                   # Individual implementation features
│       ├── feature-1.md
│       └── feature-2.md
```

## File Templates

### README.md (Component Root)

**Must contain:**
- One-line description
- Quick start (build, run, verify)
- Essential configuration
- Link to spec/

**Must NOT contain:**
- Verbose explanations
- Architecture diagrams (goes in spec/architecture.md)
- Implementation details (goes in spec/impl.md)
- Full feature lists (goes in spec/)

**Example:**
```markdown
# Component Name

One line description.

## Quick Start

\`\`\`bash
# Build
command to build

# Run
command to run

# Verify
command to verify
\`\`\`

## Documentation

See [spec/](spec/) for detailed documentation.
```

### spec/prd.md

**Must contain:**
- Product overview (1-2 sentences)
- Functional requirements (links to prds/ with short descriptions)
- Non-functional requirements (brief, 1-2 lines each)

**Must NOT contain:**
- Implementation details
- Acceptance criteria (goes in prds/)
- Technical decisions (goes in design.md)

**Example:**
```markdown
# Component PRD

## Product Overview

One line description of what the component does.

## Functional Requirements

- [FR1: Feature Name](prds/feature-name.md) – Short description of what the feature provides.
- [FR2: Another Feature](prds/another-feature.md) – Short description of what the feature provides.

## Non-Functional Requirements

### NFR1: Performance
Brief description.

### NFR2: Security
Brief description.
```

### spec/prds/feature.md

**Must contain:**
- Requirement statement
- Link to implementation feature

**Must NOT contain:**
- Implementation details
- How it works
- Code examples

**Example:**
```markdown
# FR1: Feature Name

## Requirement

What the feature must do (1-2 sentences).

## Implementation

[Feature Implementation](../impls/feature-implementation.md)
```

### spec/architecture.md

**Must contain:**
- Overview (1-2 sentences)
- Components list with ports/responsibilities
- Container/deployment structure diagram
- Integration points

**Must NOT contain:**
- Implementation details
- Step-by-step procedures
- Configuration examples

**Example:**
```markdown
# Component Architecture

## Overview

One line about the architecture approach.

## Components

### Component A (Port XXXX)
- Responsibility 1
- Responsibility 2

### Component B (Port YYYY)
- Responsibility 1
- Responsibility 2

## Container Structure

\`\`\`
[Simple ASCII diagram]
\`\`\`

## Integration

- Component A → Component B: Purpose
- External → Component A: Purpose
```

### spec/design.md

**Must contain:**
- Overview (1 sentence)
- Components (brief)
- Key decisions (why, not how)

**Must NOT contain:**
- Implementation details
- Code examples
- Step-by-step guides

**Example:**
```markdown
# Component Design

## Overview

One sentence about design approach.

## Components

- **Component A**: Brief description
- **Component B**: Brief description

## Key Decisions

- **Decision 1**: Rationale
- **Decision 2**: Rationale
```

### spec/impl.md

**Must contain:**
- Overview (1 sentence)
- Base/foundation references
- Key files list
- Requirements (versions)
- Links to implementation features with short descriptions

**Must NOT contain:**
- Detailed implementation steps
- Code examples
- Configuration details

**Example:**
```markdown
# Component Implementation

## Overview

One sentence about implementation.

## Base

- **Foundation**: Reference to base technology

## Key Files

- **file1**: Purpose
- **file2**: Purpose

## Requirements

- Tool 1 version
- Tool 2 version

## Features

- [Feature 1](impls/feature-1.md) – Short description of what the feature implements.
- [Feature 2](impls/feature-2.md) – Short description of what the feature implements.
```

### spec/impls/feature.md

**Must contain:**
- Overview (what the feature does)
- Git commits
- Motivation (why)
- Architecture diagram (if applicable)
- Implementation details (how)
- Key technical decisions
- Configuration
- Build & run commands
- Challenges & solutions
- Testing approach
- Related features
- Future enhancements

**When spec-kit was used:**
- Reference spec-kit artifacts (`artifacts/spec.md`, `artifacts/plan.md`, `artifacts/tasks.md`) for detailed requirements, user scenarios, and planning context
- Extract key information from spec.md (user scenarios, acceptance criteria, requirements) to inform implementation documentation
- Use plan.md for technical context, constitution compliance, and architecture decisions
- Use tasks.md for understanding implementation phases and dependencies

**Example:**
```markdown
# Feature: Feature Name

## Overview

What this feature does.

## Git Commits

- \`abc1234\` - Commit message
- \`def5678\` - Commit message

## Motivation

Why this feature exists.

## Implementation Details

### Sub-component 1
How it works.

### Sub-component 2
How it works.

## Key Technical Decisions

1. **Decision**: Rationale
2. **Decision**: Rationale

## Configuration

Files and settings.

## Build & Run

Commands.

## Challenges & Solutions

### Challenge 1
Problem and solution.

## Related Features

- [Other Feature](./other-feature.md)
```

### spec/impls/feature-name/ (Complex Features)

For complex features that require detailed design artifacts, use a directory structure:

```
spec/impls/feature-name/
├── feature-name.md              # Main implementation doc
└── artifacts/                   # Supporting design documents
    ├── spec.md                  # Detailed specification
    ├── plan.md                  # Implementation plan
    ├── data-model.md            # Data model and schemas
    └── openapi-*.yaml           # API contracts
```

**Main implementation file (feature-name.md) must contain:**
- Entry Points (key files with one-line descriptions)
- Behaviour (how the feature works)
- Data Model (key database schemas if applicable)
- Key Technical Decisions
- Verification (curl commands with expected responses)
- Design Artifacts (links to artifacts/ directory)
- Related Features
- Future Enhancements

**Must NOT contain:**
- Verbose explanations
- Detailed planning process
- Branch names or dates
- Session notes

**Artifacts directory (artifacts/) contains:**

**spec.md** - Detailed feature specification:
- Overview (what the feature does)
- Clarifications (questions and answers from planning)
- User Scenarios & Testing (user stories with acceptance criteria)
- Requirements (functional requirements list)
- Key Entities
- Success Criteria
- Assumptions

**plan.md** - Implementation plan:
- Overview
- Constitution Compliance Check
- Technical Context (stack, dependencies, integration points)
- Implementation Phases
- Testing Strategy
- References

**data-model.md** - Data model design:
- Entity Relationship Diagram
- Entity definitions with attributes and validation rules
- State Transitions
- Domain Models (code structs)
- DTOs
- Database Schema (SQL)
- Business Rules
- API Resource Structure

**openapi-*.yaml** - API contract (OpenAPI specification)

**When to use this structure:**
- Feature has complex data model with multiple entities
- Feature requires detailed planning artifacts
- Feature has multiple implementation phases
- You want to preserve design decisions and planning context

**When NOT to use this structure:**
- Simple features with < 3 files changed
- Minor enhancements to existing features
- Bug fixes

**Example reference:** See `platform-api/spec/impls/gateway-management/` for complete example.

## Spec-Kit Integration

**RULE**: When creating feature implementation documents, automatically check if spec-kit was used for planning:

1. **Check for spec-kit artifacts** in `<repo_root>/specs/<feature-folder>/`:
   - `spec.md` - Detailed feature specification
   - `plan.md` - Implementation plan
   - `tasks.md` - Task breakdown

2. **Copy artifacts to implementation folder** if they exist:
   - Create `spec/impls/<feature-name>/artifacts/` directory
   - Copy `spec.md`, `plan.md`, `tasks.md` from specs folder
   - **Sanitize artifacts** to remove branch-specific and session-specific information:
     - Remove branch names (e.g., `002-gateway-websockets-i`)
     - Remove dates (e.g., `**Created**: 2025-10-15`)
     - Remove status labels (e.g., `**Status**: Draft`)
     - Remove verbose planning context (e.g., `**Input**: User description...`)
     - Remove session headings (e.g., `### Session 2025-10-15`)
     - Replace spec folder paths (`/specs/002-gateway-websockets-i/`) with artifact paths (`spec/impls/<feature-name>/artifacts/`)
   - Update main implementation document to reference artifacts

3. **Create PRD document**:
   - Create `spec/prds/<feature-name>.md` following the PRD template
   - Extract requirement statement and key entities from spec.md (if available)
   - Link to implementation document in PRD
   - Update `spec/prd.md` to reference the new PRD

4. **Update cross-cutting documentation**:
   - Update `spec/architecture.md` with new components and integration points
   - Update `spec/design.md` with new key technical decisions
   - Update `spec/impl.md` with link to new feature

**Example structure when spec-kit was used:**
```
spec/impls/gateway-websocket-events/
├── gateway-websocket-events.md      # Main implementation doc
└── artifacts/                       # Spec-kit planning artifacts
    ├── spec.md                      # From specs/002-gateway-websockets-i/
    ├── plan.md                      # From specs/002-gateway-websockets-i/
    └── tasks.md                     # From specs/002-gateway-websockets-i/
```

**When to apply this rule:**
- User requests creating feature implementation document following DOCUMENTATION.md
- You complete implementation of a feature that used spec-kit planning
- You are documenting a feature that has planning artifacts in specs/ folder

**Reference:** See `platform-api/spec/impls/gateway-management/` for example with spec-kit artifacts.

## Linking Rules

1. **PRD → Implementation**: Each prds/*.md links to ONE impls/*.md
2. **Implementation → Implementation**: impls/*.md can cross-reference each other
3. **Main docs → Sub-docs**: prd.md links to prds/, impl.md links to impls/
4. **README → spec**: Component README links to spec/ directory

## Writing Rules

1. **One sentence per concept** - If you need two sentences, you're explaining too much
2. **No redundancy** - If it's in architecture.md, don't repeat in design.md
3. **No marketing speak** - "powerful", "robust", "cutting-edge" are banned
4. **Active voice** - "Starts the service" not "The service is started"
5. **Present tense** - "Does X" not "Will do X" or "Did X"
6. **No future plans in main docs** - Future enhancements go at the end of impl features
7. **Acceptance criteria only if completed** - Mark with ✅ or don't include

## Examples

### Good
> Package Thunder OAuth server and Gate App UI in a single Docker container.

### Bad
> This feature provides a comprehensive and robust solution for packaging the powerful Thunder OAuth server along with our custom-built Gate App authentication UI into a single, deployable Docker container that simplifies the deployment process.

### Good
> - **Single container**: Simplifies deployment

### Bad
> - **Single container deployment**: We chose to use a single container approach because it provides numerous benefits including simplified deployment, reduced operational overhead, easier version management, and a more streamlined developer experience.

## Reference

See [sts/spec/](../sts/spec/) for a complete example implementation.

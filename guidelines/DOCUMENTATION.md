# Documentation Guidelines

Guidelines for documenting components in the API Platform.

## Principles

1. **No unnecessary content** - Keep content concise and to the point
2. **Link requirements to implementation** - Every PRD links to its implementation feature
3. **Single source of truth** - One place for each concept, linked from multiple places
4. **Developer-first** - Quick start should get developers running immediately

## Structure

Each component must have this structure:

```
component/
├── README.md                    # Quick start + pointer to spec/
├── spec/
│   ├── README.md                # Spec overview
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
- Functional requirements (links to prds/)
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

- [FR1: Feature Name](prds/feature-name.md)
- [FR2: Another Feature](prds/another-feature.md)

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
- Links to implementation features

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

- [Feature 1](impls/feature-1.md)
- [Feature 2](impls/feature-2.md)
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

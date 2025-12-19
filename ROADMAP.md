# Skyrix Framework — Roadmap

## Project Vision

**Skyrix Framework** is an open-source, template-based Go framework for building backend services with a strong architectural foundation.

At the current stage, the framework is distributed as a **starter template**, focusing on:

- project structure and conventions,
- modular design,
- production-oriented defaults.

The long-term goal is to evolve Skyrix Framework into a **hybrid system**:

- a template-first framework for fast service creation,
- plus a set of reusable Go packages (`go get`) for stable, well-defined domains.

This roadmap describes the planned evolution.

> Note: This roadmap reflects current direction and may evolve based on real-world usage.

---

## Current State (Template-First)

**Status:** Active  
**Distribution model:** GitHub/GitLab Template Repository

At this stage, Skyrix Framework is intentionally **opinionated** and template-driven.

### What is stable now

- Canonical project structure
- Clear separation of application, engine, and domain modules
- YAML-based configuration (local / production)
- Multi-tenant–ready architecture
- HTTP and CLI entry points
- Dependency injection pattern (Wire-based)
- Production-oriented layout (configs, logging, lifecycle)

The template is designed to be **copied**, not imported as a library.

---

## Phase 1 — Framework Stabilization

**Goal:** Make the template a reliable and documented foundation for new services.

### Planned work

- Finalize and document the canonical project layout
- Improve onboarding documentation (Quick Start)
- Add reference examples (minimal HTTP service)
- Formalize conventions (naming, module boundaries)
- Document configuration structure and lifecycle

### Outcome

- New services can be created quickly and consistently
- The framework is understandable without internal context

---

## Phase 2 — Extracting Reusable Domains (Tenant & Auth)

**Goal:** Begin separating proven and stable domains into reusable components.

### Planned domains

#### Tenant
- Tenant context and resolver contracts
- Middleware and headers
- Tenant-aware request handling

#### Auth
- JWT contracts and claims
- Key management abstraction
- Authentication middleware hooks

### Key principles

- Only **stable, reusable contracts** are extracted
- Internal implementations remain flexible
- Public APIs are kept minimal

At this stage, these domains may still live in the same repository,  
but outside of `internal/`, preparing for future extraction.

---

## Phase 3 — CLI & Code Generation

**Goal:** Reduce manual setup and enforce consistency.

### Planned tooling

- CLI commands for framework operations:
  - Service initialization
  - Configuration generation
  - RSA key generation
  - Module scaffolding
- Instance / environment setup helpers
- Optional automation for local and development workflows

The CLI is intended as **developer tooling**, not a mandatory runtime dependency.

---

## Phase 4 — Go Library Packaging (`go get`)

**Goal:** Provide reusable Skyrix components as Go modules.

### Planned changes

- Extract stable packages into a separate Go module (e.g. *Skyrix Kit*)
- Apply semantic versioning
- Maintain backward compatibility for public APIs
- Keep the template dependent on the library, not vice versa

This phase will begin **only after real-world usage validates the APIs**.

---

## Design Principles

Skyrix Framework follows these core principles:

- Template-first, library-later
- Opinionated where it matters
- Minimal public surface
- Clear boundaries between framework and application
- Real-world driven evolution

The framework is designed to grow **with usage**, not ahead of it.

---

## Non-Goals

The following are explicitly **not** goals at this stage:

- Becoming a generic “one-size-fits-all” framework
- Freezing public APIs too early
- Forcing library usage over templates
- Competing with low-level Go libraries

---

## Status Summary

- **Current:** Template-based open-source Go framework  
- **Next:** Stabilization and domain extraction  
- **Future:** Hybrid framework (Template + Go packages)

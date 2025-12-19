# Skyrix Framework — Architecture & Balance Pattern

## Overview

Skyrix Framework is an open-source, template-based Go framework designed for building
long-lived backend services with a strong architectural foundation.

The framework is built around a **Balance Pattern** that explicitly separates
stable infrastructure concerns from evolving business logic, while keeping
the system pragmatic and product-driven.

This document describes the core architecture, package taxonomy,
and the data access strategy used throughout the framework.

---

## Core Architectural Idea

Skyrix Framework follows a layered, responsibility-driven architecture with
clear boundaries between framework internals and domain logic.

The system is intentionally split into:

- **Framework Internals** — technical and infrastructural mechanisms
- **Domain Packages** — business logic and domain behavior
- **Application Assembly** — composition and wiring
- **Entry Points** — executable boundaries

Each layer evolves at a different pace and serves a distinct purpose.

---

## High-Level Project Structure

Typical layout (template-first):

- `cmd/` — application entry points (http, console)
- `config/` — environment-specific configuration (YAML)
- `docs/` — documentation (optional)
- `internal/` — framework internals and domain modules

Entry points contain no business logic; assembly is explicit; domains stay isolated.

---

## Two Types of Packages (Key Concept)

Skyrix Framework distinguishes **two categories of packages**:

1. **Internal packages (framework / engine / kernel)**
2. **Domain packages (business logic)**

This separation is a foundation of the Balance Pattern.

---

## 1) Internal Packages (Framework / Engine)

**Location:** `internal/kernel`, `internal/engine`, `internal/router`, `internal/providers`, etc.

Internal packages represent the **platform layer**:
- domain-agnostic infrastructure
- cross-cutting concerns
- lifecycle and transport plumbing

### Characteristics

- Technical responsibilities (db, redis, transactions, http wiring, middleware)
- Domain-agnostic by design
- Hidden behind `internal/` to prevent accidental reuse
- Candidates for future extraction into libraries after validation

---

## 2) Domain Packages (Business)

**Location:** `internal/domain/*`

Domain packages contain business logic and domain behavior.
Each domain is self-contained and owns its internal structure.

Typical domain structure:

- `entity/` — domain entities (write model)
- `repository/` — persistence layer for domain operations
- `services/` — domain services and business rules
- `provider.go` — module integration point for assembly/DI

Domains are free to evolve quickly without forcing framework-level API changes.

---

## Application Assembly & Providers

**Location:** `internal/providers`, plus assembly wiring in `cmd/*/wire.go`

Assembly is responsible for:
- composition of domains and internal components,
- initialization order,
- dependency injection wiring (Wire-based),
- HTTP routes/handlers registration.

Providers act as explicit integration points between internal and domain packages.

---

## Entry Points

**Location:** `cmd/http`, `cmd/console`

Entry points define:
- how the application starts,
- which runtime is used (HTTP server / console),
- what gets assembled.

They contain no domain logic.

---

## The Balance Pattern

### The Problem

Systems usually fail by choosing an extreme:

- overly abstract frameworks with frozen APIs, or
- unstructured codebases that degrade over time.

Skyrix Framework avoids both through the **Balance Pattern**.

---

## What Is the Balance Pattern?

The Balance Pattern is the principle of placing stability and change
into different layers:

- stable structure enables controlled change,
- change remains local to domains,
- reusable parts are extracted only after proof in real usage.

---

## Zones of Stability

These areas change slowly and define the framework:

- project structure and conventions
- lifecycle and bootstrapping
- configuration model
- cross-cutting concerns (logging, auth context, tenant resolution)

These form the **Framework Core**.

---

## Zones of Change

These areas evolve quickly:

- business rules
- domain modules
- integrations
- service-specific behavior

They live in **domain packages and assembly**, not in the core.

---

## Template-first, Library-later

Skyrix Framework intentionally starts as a **template**:

- code is copied, not imported,
- public APIs are not frozen prematurely,
- architecture is validated by real usage.

Stable components may later be extracted into Go modules (`go get`).

---

## Dependency Injection as a Balance Tool

Wire-based DI is used to:
- make dependencies explicit,
- control composition at the assembly layer,
- avoid hidden global state.

DI belongs to assembly, not domain.

---

## Multi-Tenancy as a First-Class Concern

Multi-tenancy is treated as a cross-cutting architectural concern:

- tenant resolution happens early in the request lifecycle
- tenant context flows through the system
- tenant routing/schema logic stays centralized

This prevents tenant logic from leaking into domain code.

---

## Data Access Strategy & CQRS Balance

Skyrix Framework does not enforce a strict CQRS implementation,
but follows a **pragmatic CQRS-inspired approach** to data access.

The goal is to balance:
- developer productivity,
- performance,
- safety,
- and long-term maintainability.

### Architectural Style

Skyrix Framework follows a layered DDD-inspired architecture with a pragmatic,
performance-aware approach to reads.

- **Write path:** domain-oriented, transactional, ORM-backed, focused on enforcing invariants.
- **Read path:** CQRS-inspired, optimized for performance and clarity; complex reads may use raw SQL to build
  **read models / projections** that combine multiple tables into a single logical representation
  aligned with a specific API use case (not a domain aggregate).

This approach keeps domain logic clean while allowing production-grade query performance.

---

## Command vs Query Responsibilities

The framework conceptually distinguishes between:

- **Commands** — operations that change state
- **Queries** — operations that read data

This distinction is logical, not infrastructural.
Commands and queries may coexist in the same domain module,
but they follow different data access rules.

---

## Commands (Write Path)

Commands are expected to:
- modify domain state,
- enforce business rules,
- operate within transactional boundaries.

### Command Data Access Rules

- ORM is preferred for write operations
- Entities are the primary write model
- Transactions are managed by the framework
- Business invariants live in domain services

---

## Queries (Read Path)

Queries are treated as read-only operations and are optimized for performance and clarity.

### Query Data Access Rules

- Simple queries may use ORM repositories
- Complex queries may use raw SQL
- Raw SQL queries should:
  - be explicit and readable,
  - be optimized for the specific use case,
  - avoid unnecessary ORM abstractions.

To maintain safety and consistency:
- raw SQL is executed through database/ORM wrappers,
- parameter binding and connection handling remain centralized,
- results are mapped into DTOs/read models.

---

## ORM as a Safety Boundary

ORM is treated as a safety boundary, not as a universal abstraction.

ORM responsibilities:
- connection and transaction management,
- parameter binding,
- protection against SQL injection,
- consistent error handling.

ORM is not required to express every complex query.

---

## Read Models vs Write Models

Skyrix Framework allows using different models for reads and writes:

- **Write models:** domain entities, ORM-managed, enforce invariants
- **Read models:** DTOs/projections, SQL-backed, optimized for API responses

This separation improves:
- performance,
- clarity,
- evolution of read APIs without breaking domain logic.

---

## Why This Is Not “Strict CQRS”

Skyrix Framework intentionally avoids:
- mandatory command/query buses,
- duplicated infrastructure layers,
- artificial separation that adds complexity without value.

Instead, it provides:
- conceptual CQRS boundaries,
- clear rules for data access,
- freedom to evolve toward stricter CQRS when justified.

---

## Architectural Non-Goals

The framework intentionally avoids:

- enforcing strict DDD terminology
- providing universal abstractions
- hiding complexity behind generic interfaces
- competing with low-level Go libraries

---

## Summary

Skyrix Framework architecture is built on explicit boundaries:

- internal vs domain packages,
- stability vs change,
- template vs library.

This balance allows the framework to evolve naturally without sacrificing
clarity or long-term maintainability.

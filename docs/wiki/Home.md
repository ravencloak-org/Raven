# Raven Platform Wiki

Welcome to the Raven platform wiki -- the central knowledge base for the project.

## Quick Links

- [Architecture Overview](Architecture-Overview)
- [Tech Stack](Tech-Stack)
- [Data Model](Data-Model)
- [API Reference](API-Reference)
- [Deployment Guide](Deployment-Guide)
- [Monetization Strategy](Monetization-Strategy)
- [Roadmap](Roadmap)
- [Hardware Requirements](Hardware-Requirements)

## What is Raven?

Raven is an open-source, multi-tenant knowledge base platform that lets organizations ingest documents, websites, and media into searchable knowledge bases. Users interact through three channels: an embeddable chatbot, a voice agent, and WebRTC/WhatsApp voice calls.

**Hierarchy:** Organization > Workspace > Knowledge Base

**Stack:** Go (Echo) + Python AI Worker + PostgreSQL 18 (pgvector + ParadeDB) + Vue.js + Keycloak

## Getting Started

See the [README](https://github.com/ravencloak-org/Raven) for project overview and quick start.

For the complete design specification, see [`docs/superpowers/specs/2026-03-27-raven-platform-design-final.md`](https://github.com/ravencloak-org/Raven/blob/main/docs/superpowers/specs/2026-03-27-raven-platform-design-final.md).

# Raven Enterprise Edition

Raven uses an open-core model: the core platform is Apache 2.0 (free for everyone), while enterprise features live in `ee/` subdirectories under a separate license. Enterprise features require a valid license key for production use — see [ee-LICENSE](./ee-LICENSE) for full terms.

Enterprise features are distributed across the codebase within each service:

| Service | Path | Language | Features |
|---------|------|----------|----------|
| Go API backend | [`internal/ee/`](./internal/ee/README.md) | Go | Licensing, lead intel, webhooks, connectors, security, audit, SSO, analytics |
| Vue.js frontend | [`frontend/src/ee/`](./frontend/src/ee/README.md) | TypeScript/Vue | Lead UI, connector config, analytics dashboards, security management |
| Python AI worker | [`ai-worker/raven_worker/ee/`](./ai-worker/raven_worker/ee/README.md) | Python | Connector integration, lead scoring ML |

## License

All code in `ee/` directories is licensed under the [Raven Enterprise License](./ee-LICENSE).
The rest of the codebase is licensed under [Apache 2.0](./LICENSE).

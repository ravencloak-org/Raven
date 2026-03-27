# Raven Enterprise Edition

Enterprise features are distributed across the codebase in `ee/` subdirectories within each service:

| Service | Path | Language |
|---------|------|----------|
| Go API backend | `internal/ee/` | Go |
| Vue.js frontend | `frontend/src/ee/` | TypeScript/Vue |
| Python AI worker | `ai-worker/raven_worker/ee/` | Python |

## License

All code in `ee/` directories is licensed under the [Raven Enterprise License](./ee-LICENSE).
The rest of the codebase is licensed under [Apache 2.0](./LICENSE).

See each service's `ee/README.md` for the feature list and tier mapping.

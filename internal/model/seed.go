package model

// SeedResult is the response from the seed endpoint.
type SeedResult struct {
	OrgID             string `json:"org_id"`
	WorkspaceID       string `json:"workspace_id"`
	KBID              string `json:"kb_id"`
	DocumentsEnqueued int    `json:"documents_enqueued"`
	PipelineStatus    string `json:"pipeline_status"`
}

// Package resilience will house circuit-breaker, retry, bulkhead, and
// timeout middleware for outbound calls (gRPC AI worker, Asynq enqueue).
//
// Placeholder file that anchors the github.com/sony/gobreaker/v2 dependency
// in go.mod until Task 2 populates the full implementation.
package resilience

import _ "github.com/sony/gobreaker/v2" // anchor dep — replaced in Task 2

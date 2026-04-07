package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/tts"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// TTSServicer is the interface the handler requires from the TTS service layer.
type TTSServicer interface {
	Synthesize(ctx context.Context, text string, opts tts.SynthesizeOpts) (io.ReadCloser, error)
	ProviderName() string
}

// TTSHandler handles HTTP requests for text-to-speech synthesis.
type TTSHandler struct {
	svc TTSServicer
}

// NewTTSHandler creates a new TTSHandler.
func NewTTSHandler(svc TTSServicer) *TTSHandler {
	return &TTSHandler{svc: svc}
}

// synthesizeRequest is the JSON body for POST /orgs/:org_id/tts.
type synthesizeRequest struct {
	Text       string `json:"text" binding:"required"`
	VoiceID    string `json:"voice_id,omitempty"`
	Language   string `json:"language,omitempty"`
	Format     string `json:"format,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// Synthesize handles POST /v1/orgs/:org_id/tts.
//
// @Summary     Text-to-speech synthesis
// @Description Converts text to audio using the configured TTS provider. Supports sentence-boundary dispatch for low latency.
// @Tags        tts
// @Accept      json
// @Produce     application/octet-stream
// @Param       org_id  path   string             true "Organisation ID"
// @Param       request body   synthesizeRequest  true "TTS request"
// @Success     200 {file} binary
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/tts [post]
func (h *TTSHandler) Synthesize(c *gin.Context) {
	_, ok := extractOrgID(c)
	if !ok {
		return
	}

	var req synthesizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	var format tts.AudioFormat
	switch req.Format {
	case "pcm":
		format = tts.AudioFormatPCM
	case "opus":
		format = tts.AudioFormatOPUS
	case "mp3", "":
		format = tts.AudioFormatMP3
	default:
		_ = c.Error(apierror.NewBadRequest("unsupported format: must be mp3, pcm, or opus"))
		c.Abort()
		return
	}

	opts := tts.SynthesizeOpts{
		VoiceID:    req.VoiceID,
		Language:   req.Language,
		Format:     format,
		SampleRate: req.SampleRate,
	}

	rc, err := h.svc.Synthesize(c.Request.Context(), req.Text, opts)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	defer func() {
		_ = rc.Close()
	}()

	contentType := resolveContentType(format)
	c.Header("Content-Type", contentType)
	c.Header("X-TTS-Provider", h.svc.ProviderName())
	c.Status(http.StatusOK)

	if _, err := io.Copy(c.Writer, rc); err != nil {
		// The response headers are already sent, so we cannot send a JSON error.
		// Log the error; the client will see a truncated response.
		_ = c.Error(apierror.NewInternal("failed to stream TTS audio"))
	}
}

// resolveContentType maps AudioFormat to an HTTP Content-Type.
func resolveContentType(f tts.AudioFormat) string {
	switch f {
	case tts.AudioFormatPCM:
		return "audio/pcm"
	case tts.AudioFormatOPUS:
		return "audio/ogg"
	case tts.AudioFormatMP3:
		return "audio/mpeg"
	default:
		return "audio/mpeg"
	}
}

package api_test

import (
	"net/http"
	"testing"
)

// Инвариант (план 46, A6): битрейт звука по умолчанию — 192 кбит/с; панель,
// которая поле звука не шлёт (до 0.18.37), его не сбрасывает, а явное
// значение сохраняется.
func TestAdminCompressionAudioBitrate(t *testing.T) {
	srv, _, _, blobs := setupAPI(t)
	admin := adminToken(t, srv)

	cs, err := blobs.LoadCompressionSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cs.AudioBitrateKbps != 192 {
		t.Fatalf("default audio bitrate: %d want 192", cs.AudioBitrateKbps)
	}

	old := map[string]any{
		"photo_max_px": 2048, "photo_quality": 80, "video_max_height": 1080,
		"video_bitrate_kbps": 6000, "attachment_max_bytes": 104857600,
	}
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/compression", admin, old); rec.Code != http.StatusOK {
		t.Fatalf("save without audio: %d %s", rec.Code, rec.Body.String())
	}
	if cs, _ = blobs.LoadCompressionSettings(t.Context()); cs.AudioBitrateKbps != 192 {
		t.Fatalf("audio bitrate after old panel: %d want 192", cs.AudioBitrateKbps)
	}

	old["audio_bitrate_kbps"] = 96
	if rec := doJSON(t, srv, http.MethodPut, "/api/v1/admin/compression", admin, old); rec.Code != http.StatusOK {
		t.Fatalf("save with audio: %d %s", rec.Code, rec.Body.String())
	}
	if cs, _ = blobs.LoadCompressionSettings(t.Context()); cs.AudioBitrateKbps != 96 {
		t.Fatalf("audio bitrate: %d want 96", cs.AudioBitrateKbps)
	}
}

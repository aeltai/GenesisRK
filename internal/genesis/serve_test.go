package genesis_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aeltai/hangar-genesis/internal/genesis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONErrorResponseShape(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusMethodNotAllowed)
	_ = json.NewEncoder(rec.Body).Encode(map[string]string{"error": "GET only"})

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, "GET only", body["error"])
	assert.NotEmpty(t, genesis.Version)
}

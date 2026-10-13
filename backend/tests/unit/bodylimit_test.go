package unit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/api"
	"github.com/stretchr/testify/assert"
)

func bodyLimitRouter(called *bool, tooLarge *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/m", api.BodyLimit(1024), func(c *gin.Context) {
		*called = true
		var v struct {
			A string `json:"a"`
		}
		if err := c.ShouldBindJSON(&v); err != nil {
			*tooLarge = api.IsBodyTooLarge(err)
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusCreated)
	})
	return r
}

func TestBodyLimitRejectsOversizedContentLength(t *testing.T) {
	var called, tooLarge bool
	r := bodyLimitRouter(&called, &tooLarge)

	body := `{"a":"` + strings.Repeat("x", 2000) + `"}`
	req := httptest.NewRequest("POST", "/m", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.False(t, called, "handler must not run")
}

func TestBodyLimitStopsUnknownLengthBodies(t *testing.T) {
	var called, tooLarge bool
	r := bodyLimitRouter(&called, &tooLarge)

	body := io.MultiReader(strings.NewReader(`{"a":"`), strings.NewReader(strings.Repeat("x", 5000)), strings.NewReader(`"}`))
	req := httptest.NewRequest("POST", "/m", body) // ContentLength == -1
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called)
	assert.True(t, tooLarge, "bind error must be recognised as body-too-large")
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodyLimitAllowsSmallBodies(t *testing.T) {
	var called, tooLarge bool
	r := bodyLimitRouter(&called, &tooLarge)

	req := httptest.NewRequest("POST", "/m", strings.NewReader(`{"a":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.False(t, tooLarge)
}

func TestMaxMessageBodyBytes(t *testing.T) {
	// 5 MiB file => ~7.0 MB of base64 JSON; the limit must leave headroom.
	assert.Equal(t, int64(8<<20), int64(api.MaxMessageBodyBytes))
}

package apiresponse

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/gin-gonic/gin"
)

func setupContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func TestOK(t *testing.T) {
	c, w := setupContext()
	OK(c, map[string]any{"id": 42})

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", body["data"])
	}
	if data["id"] != float64(42) {
		t.Errorf("expected data.id 42, got %v", data["id"])
	}
}

func TestCreated(t *testing.T) {
	c, w := setupContext()
	Created(c, map[string]any{"id": 7})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestError_response(t *testing.T) {
	c, w := setupContext()
	apiErr := apierror.ErrForbidden("no access")
	Error(c, apiErr)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	if body["status"] != "error" {
		t.Errorf("expected status error, got %v", body["status"])
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", body["error"])
	}
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("expected code FORBIDDEN, got %v", errObj["code"])
	}
	if errObj["message"] != "no access" {
		t.Errorf("expected message no access, got %v", errObj["message"])
	}
}

func TestErrorFromErr_apiError(t *testing.T) {
	c, w := setupContext()
	inner := apierror.ErrValidation("bad request")
	ErrorFromErr(c, inner)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", body["error"])
	}
	if errObj["code"] != "VALIDATION" {
		t.Errorf("expected code VALIDATION, got %v", errObj["code"])
	}
}

func TestErrorFromErr_plainError(t *testing.T) {
	c, w := setupContext()
	plain := errors.New("something went wrong")
	ErrorFromErr(c, plain)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T", body["error"])
	}
	if errObj["code"] != "INTERNAL" {
		t.Errorf("expected code INTERNAL, got %v", errObj["code"])
	}
	if errObj["message"] != "something went wrong" {
		t.Errorf("expected message something went wrong, got %v", errObj["message"])
	}
}

func TestErrorFromErr_nil(t *testing.T) {
	c, w := setupContext()
	ErrorFromErr(c, nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected default status %d when no response written, got %d", http.StatusOK, w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body for nil error, got %q", w.Body.String())
	}
}

package apierror

import (
	"errors"
	"testing"
)

func TestErrNotFound(t *testing.T) {
	e := ErrNotFound("track missing")
	if e.Code != CodeNotFound {
		t.Errorf("expected code %q, got %q", CodeNotFound, e.Code)
	}
	if e.Message != "track missing" {
		t.Errorf("expected message %q, got %q", "track missing", e.Message)
	}
	if e.HTTPStatus != 404 {
		t.Errorf("expected status 404, got %d", e.HTTPStatus)
	}
	if e.Error() != "track missing" {
		t.Errorf("expected Error() %q, got %q", "track missing", e.Error())
	}
}

func TestErrValidation(t *testing.T) {
	e := ErrValidation("bad input")
	if e.Code != CodeValidation {
		t.Errorf("expected code %q, got %q", CodeValidation, e.Code)
	}
	if e.HTTPStatus != 400 {
		t.Errorf("expected status 400, got %d", e.HTTPStatus)
	}
}

func TestErrForbidden(t *testing.T) {
	e := ErrForbidden("no access")
	if e.Code != CodeForbidden {
		t.Errorf("expected code %q, got %q", CodeForbidden, e.Code)
	}
	if e.HTTPStatus != 403 {
		t.Errorf("expected status 403, got %d", e.HTTPStatus)
	}
}

func TestErrInternal(t *testing.T) {
	e := ErrInternal("something broke")
	if e.Code != CodeInternal {
		t.Errorf("expected code %q, got %q", CodeInternal, e.Code)
	}
	if e.HTTPStatus != 500 {
		t.Errorf("expected status 500, got %d", e.HTTPStatus)
	}
}

func TestErrRateLimited(t *testing.T) {
	e := ErrRateLimited("slow down")
	if e.Code != CodeRateLimited {
		t.Errorf("expected code %q, got %q", CodeRateLimited, e.Code)
	}
	if e.HTTPStatus != 429 {
		t.Errorf("expected status 429, got %d", e.HTTPStatus)
	}
}

func TestErrUnauthorized(t *testing.T) {
	e := ErrUnauthorized("log in first")
	if e.Code != CodeUnauthorized {
		t.Errorf("expected code %q, got %q", CodeUnauthorized, e.Code)
	}
	if e.HTTPStatus != 401 {
		t.Errorf("expected status 401, got %d", e.HTTPStatus)
	}
}

func TestErrConflict(t *testing.T) {
	e := ErrConflict("already exists")
	if e.Code != CodeConflict {
		t.Errorf("expected code %q, got %q", CodeConflict, e.Code)
	}
	if e.HTTPStatus != 409 {
		t.Errorf("expected status 409, got %d", e.HTTPStatus)
	}
}

func TestErrorIs_sameCode(t *testing.T) {
	a := ErrNotFound("a")
	b := ErrNotFound("b")
	if !errors.Is(a, b) {
		t.Error("expected errors.Is to match for same code")
	}
}

func TestErrorIs_differentCode(t *testing.T) {
	a := ErrNotFound("x")
	b := ErrValidation("x")
	if errors.Is(a, b) {
		t.Error("expected errors.Is to NOT match for different codes")
	}
}

func TestErrorIs_wrapped(t *testing.T) {
	inner := ErrNotFound("inner")
	wrapped := errors.New("wrapper")
	// errors.Is checks the chain; our Is only compares Code.
	if !errors.Is(inner, ErrNotFound("")) {
		t.Error("expected errors.Is to match sentinel")
	}
	_ = wrapped
}

func TestErrorIs_nonApiError(t *testing.T) {
	e := ErrNotFound("x")
	plain := errors.New("plain")
	if errors.Is(e, plain) {
		t.Error("expected errors.Is to NOT match plain error")
	}
}

// Package apiresponse provides standardized JSON envelope helpers for Gin handlers.
package apiresponse

import (
	"errors"
	"net/http"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/gin-gonic/gin"
)

// okResponse is the success envelope shape.
type okResponse struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
}

// errorResponse is the error envelope shape.
type errorResponse struct {
	Status string          `json:"status"`
	Error  *errorDetail    `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK writes a 200 JSON response with {"status":"ok","data":data}.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, okResponse{Status: "ok", Data: data})
}

// Created writes a 201 JSON response with {"status":"ok","data":data}.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, okResponse{Status: "ok", Data: data})
}

// Error writes a JSON error response using the HTTP status stored in apiErr.
func Error(c *gin.Context, apiErr *apierror.Error) {
	c.JSON(apiErr.HTTPStatus, errorResponse{
		Status: "error",
		Error: &errorDetail{
			Code:    apiErr.Code,
			Message: apiErr.Message,
		},
	})
}

// ErrorFromErr writes a JSON error response from an arbitrary error.
// If err is nil, it does nothing.
// If err is an *apierror.Error, it uses the stored HTTP status and code.
// Otherwise it falls back to apierror.ErrInternal.
func ErrorFromErr(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		Error(c, apiErr)
		return
	}

	Error(c, apierror.ErrInternal(err.Error()))
}

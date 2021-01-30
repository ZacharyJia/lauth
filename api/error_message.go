package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/macrat/ldapin/metrics"
)

type ErrorMessage struct {
	Err          error    `json:"-"`
	RedirectURI  *url.URL `json:"-"`
	ResponseType string   `json:"-"`
	State        string   `json:"state,omitempty"`
	Reason       string   `json:"error"`
	Description  string   `json:"error_description,omitempty"`
	ErrorURI     string   `json:"error_uri,omitempty"`
}

func (msg ErrorMessage) Unwrap() error {
	return msg.Err
}

func (msg ErrorMessage) Error() string {
	if msg.State == "" {
		return fmt.Sprintf("%s: %s", msg.Reason, msg.Description)
	} else {
		return fmt.Sprintf("%s(%s): %s", msg.Reason, msg.State, msg.Description)
	}
}

func (msg ErrorMessage) Redirect(c *gin.Context) {
	if msg.RedirectURI == nil || msg.RedirectURI.String() == "" || !msg.RedirectURI.IsAbs() {
		c.HTML(http.StatusBadRequest, "error.tmpl", gin.H{
			"error": msg,
		})
		return
	}

	resp := make(url.Values)
	if msg.State != "" {
		resp.Set("state", msg.State)
	}

	resp.Set("error", msg.Reason)
	if msg.Description != "" {
		resp.Set("error_description", msg.Description)
	}

	if msg.ResponseType != "code" && msg.ResponseType != "" {
		msg.RedirectURI.Fragment = resp.Encode()
	} else {
		msg.RedirectURI.RawQuery = resp.Encode()
	}
	c.Redirect(http.StatusFound, msg.RedirectURI.String())
}

func (msg ErrorMessage) JSON(c *gin.Context) {
	switch msg.Reason {
	case "server_error":
		c.JSON(http.StatusInternalServerError, msg)
	case "invalid_token":
		c.JSON(http.StatusForbidden, msg)
	default:
		c.JSON(http.StatusBadRequest, msg)
	}
}

func (msg ErrorMessage) Report(c *metrics.Context) {
	c.Set("error", msg.Reason)
	c.Set("error_description", msg.Description)
}

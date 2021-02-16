package api_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/macrat/lauth/api"
	"github.com/macrat/lauth/config"
	"github.com/macrat/lauth/testutil"
)

func authzEndpointCommonTests(t *testing.T, c *config.Config) []testutil.RedirectTest {
	return []testutil.RedirectTest{
		{
			Name:        "without any query",
			Request:     url.Values{},
			Code:        http.StatusBadRequest,
			HasLocation: false,
		},
		{
			Name: "missing client_id",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"response_type": {"code"},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
		},
		{
			Name: "missing response_type",
			Request: url.Values{
				"redirect_uri": {"http://some-client.example.com/callback"},
				"client_id":    {"some_client_id"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"unsupported_response_type"},
				"error_description": {"response_type is required"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "unknown response_type",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code hogefuga"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query:       url.Values{},
			Fragment: url.Values{
				"error":             {"unsupported_response_type"},
				"error_description": {"response_type \"hogefuga\" is not supported"},
			},
		},
		{
			Name: "relative redirect_uri",
			Request: url.Values{
				"redirect_uri":  {"/invalid/relative/url"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
		},
		{
			Name: "not registered client_id",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"another_client_id"},
				"response_type": {"code"},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
		},
		{
			Name: "invalid code (can't parse)",
			Request: url.Values{
				"redirect_uri":  {"http://other-site.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
		},
		{
			Name: "missing redirect_uri",
			Request: url.Values{
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
		},
		{
			Name: "invalid redirect_uri",
			Request: url.Values{
				"redirect_uri":  {"this is invalid url::"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
			Query:       url.Values{},
			Fragment:    url.Values{},
		},
		{
			Name: "disallowed hybrid flow",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code token"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query:       url.Values{},
			Fragment: url.Values{
				"error":             {"unsupported_response_type"},
				"error_description": {"implicit/hybrid flow is disallowed"},
			},
		},
		{
			Name: "request object / can't parse",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"request":       {"invalid request"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request_object"},
				"error_description": {"failed to decode or validation request object"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "request object / empty request",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"request":       {testutil.SomeClientRequestObject(t, map[string]interface{}{})},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request_object"},
				"error_description": {"failed to decode or validation request object"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "request object / mismatch some values",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"scope":         {"openid profile"},
				"state":         {"this is state"},
				"nonce":         {"this is nonce"},
				"max_age":       {"123"},
				"prompt":        {"login"},
				"login_hint":    {"macrat"},
				"request": {testutil.SomeClientRequestObject(t, map[string]interface{}{
					"iss":           "some_client_id",
					"aud":           c.Issuer.String(),
					"client_id":     "another_client_id",
					"response_type": "token",
					"redirect_uri":  "http://another-client.example.com/callback",
					"scope":         "openid profile email",
					"state":         "this is another state",
					"nonce":         "this is nonce",
					"max_age":       123,
					"prompt":        "login",
					"login_hint":    "macrat",
				})},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request_object"},
				"error_description": {"mismatch query parameter and request object: response_type, client_id, redirect_uri, scope, state"},
				"state":             {"this is state"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "request object / mismatch another some values",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"scope":         {"openid profile"},
				"state":         {"this is state"},
				"nonce":         {"this is nonce"},
				"max_age":       {"123"},
				"prompt":        {"login"},
				"login_hint":    {"macrat"},
				"request": {testutil.SomeClientRequestObject(t, map[string]interface{}{
					"iss":           "some_client_id",
					"aud":           c.Issuer.String(),
					"client_id":     "some_client_id",
					"response_type": "code",
					"redirect_uri":  "http://some-client.example.com/callback",
					"scope":         "openid profile",
					"state":         "this is state",
					"nonce":         "this is anothernonce",
					"max_age":       42,
					"prompt":        "consent",
					"login_hint":    "j.smith",
				})},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request_object"},
				"error_description": {"mismatch query parameter and request object: nonce, max_age, prompt, login_hint"},
				"state":             {"this is state"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "request object / invalid redirect_uri",
			Request: url.Values{
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"request": {testutil.SomeClientRequestObject(t, map[string]interface{}{
					"iss":          "some_client_id",
					"aud":          c.Issuer.String(),
					"redirect_uri": "this is invalid url::",
				})},
			},
			Code:        http.StatusBadRequest,
			HasLocation: false,
			Query:       url.Values{},
			Fragment:    url.Values{},
		},
		{
			Name: "request object / set both of prompt=none and prompt=login",
			Request: url.Values{
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"request": {testutil.SomeClientRequestObject(t, map[string]interface{}{
					"iss":          "some_client_id",
					"aud":          c.Issuer.String(),
					"redirect_uri": "http://some-client.example.com/callback",
					"prompt":       "none login",
				})},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request"},
				"error_description": {"prompt=none can't use same time with login, select_account, or consent"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "request_uri is not supported",
			Request: url.Values{
				"redirect_uri": {"http://some-client.example.com/callback"},
				"client_id":    {"some_client_id"},
				"request_uri":  {"http://some-client.example.com/request.jwt"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error": {"request_uri_not_supported"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "missing nonce in implicit flow",
			Request: url.Values{
				"redirect_uri":  {"http://implicit-client.example.com/callback"},
				"client_id":     {"implicit_client_id"},
				"response_type": {"token id_token"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query:       url.Values{},
			Fragment: url.Values{
				"error":             {"invalid_request"},
				"error_description": {"nonce is required in the implicit/hybrid flow of OpenID Connect"},
			},
		},
		{
			Name: "can't use both prompt of none and login",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"prompt":        {"none login"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request"},
				"error_description": {"prompt=none can't use same time with login, select_account, or consent"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "can't use both prompt of none and consent",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"prompt":        {"consent none"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request"},
				"error_description": {"prompt=none can't use same time with login, select_account, or consent"},
			},
			Fragment: url.Values{},
		},
		{
			Name: "can't use both prompt of none and select_account",
			Request: url.Values{
				"redirect_uri":  {"http://some-client.example.com/callback"},
				"client_id":     {"some_client_id"},
				"response_type": {"code"},
				"prompt":        {"none select_account"},
			},
			Code:        http.StatusFound,
			HasLocation: true,
			Query: url.Values{
				"error":             {"invalid_request"},
				"error_description": {"prompt=none can't use same time with login, select_account, or consent"},
			},
			Fragment: url.Values{},
		},
	}
}

func TestSSOLogin(t *testing.T) {
	env := testutil.NewAPITestEnvironment(t)

	session, err := env.API.MakeLoginSession("::1", "some_client_id")
	if err != nil {
		t.Fatalf("failed to create session token: %s", err)
	}

	t.Log("---------- first login ----------")

	resp := env.Post("/authz", "", url.Values{
		"redirect_uri":  {"http://some-client.example.com/callback"},
		"client_id":     {"some_client_id"},
		"response_type": {"code"},
		"session":       {session},
		"username":      {"macrat"},
		"password":      {"foobar"},
	})
	if resp.Code != http.StatusFound {
		t.Fatalf("unexpected status code on first login: %d", resp.Code)
	}
	rawCookie, ok := resp.Header()["Set-Cookie"]
	if !ok {
		t.Fatalf("cookies for SSO was not found")
	}

	cookie, _ := (&http.Request{Header: http.Header{"Cookie": rawCookie}}).Cookie(api.SSO_TOKEN_COOKIE)

	ssoToken, err := env.API.TokenManager.ParseSSOToken(cookie.Value)
	if err != nil {
		t.Errorf("failed to parse token in cookie: %s", err)
	} else if err := ssoToken.Validate(env.API.Config.Issuer); err != nil {
		t.Errorf("token in cookie is invalid: %s", err)
	}

	t.Log("---------- login with SSO token ----------")

	params := url.Values{
		"redirect_uri":  {"http://some-client.example.com/callback"},
		"client_id":     {"some_client_id"},
		"response_type": {"code"},
	}
	req, _ := http.NewRequest("GET", "/authz?"+params.Encode(), nil)
	for _, c := range rawCookie {
		req.Header.Add("Cookie", c)
	}

	resp = env.DoRequest(req)
	if resp.Code != http.StatusFound {
		t.Fatalf("unexpected status code on login with SSO token: %d", resp.Code)
	}

	location, err := url.Parse(resp.Header().Get("Location"))
	if err != nil {
		t.Errorf("failed to parse location: %s", err)
	}
	code, err := env.API.TokenManager.ParseCode(location.Query().Get("code"))
	if err != nil {
		t.Errorf("failed to parse code: %s", err)
	} else if err = code.Validate(env.API.Config.Issuer); err != nil {
		t.Errorf("respond code is invalid: %s", err)
	} else if code.AuthTime != ssoToken.AuthTime {
		t.Errorf("auth_time is not match: sso_token=%d != code=%d", ssoToken.AuthTime, code.AuthTime)
	} else if code.Subject != ssoToken.Subject {
		t.Errorf("auth_time is not match: sso_token=%s != code=%s", ssoToken.Subject, code.Subject)
	}

	t.Log("---------- show consent prompt with SSO token ----------")

	params = url.Values{
		"redirect_uri":  {"http://some-client.example.com/callback"},
		"client_id":     {"some_client_id"},
		"response_type": {"code"},
		"prompt":        {"consent"},
	}
	req, _ = http.NewRequest("GET", "/authz?"+params.Encode(), nil)
	for _, c := range rawCookie {
		req.Header.Add("Cookie", c)
	}

	resp = env.DoRequest(req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code on prompt=consent with SSO token: %d", resp.Code)
	}

	inputs, err := testutil.FindInputsByHTML(resp.Body)
	if err != nil {
		t.Fatalf("failed to parse consent page: %s", err)
	}

	if _, ok := inputs["username"]; ok {
		t.Errorf("expected consent page but got username input")
	}

	if _, ok := inputs["password"]; ok {
		t.Errorf("expected consent page but got password input")
	}

	t.Log("---------- login via consent prompt ----------")
	params = url.Values{}
	for k, v := range inputs {
		params.Add(k, v)
	}

	req, _ = http.NewRequest("POST", "/authz", strings.NewReader(params.Encode()))
	for _, c := range rawCookie {
		req.Header.Add("Cookie", c)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp = env.DoRequest(req)
	if resp.Code != http.StatusFound {
		t.Fatalf("unexpected status code on login via consent prompt: %d", resp.Code)
	}

	location, err = url.Parse(resp.Header().Get("Location"))
	if err != nil {
		t.Errorf("failed to parse location: %s", err)
	}
	code, err = env.API.TokenManager.ParseCode(location.Query().Get("code"))
	if err != nil {
		t.Errorf("failed to parse code: %s", err)
	} else if err = code.Validate(env.API.Config.Issuer); err != nil {
		t.Errorf("respond code is invalid: %s", err)
	} else if code.AuthTime != ssoToken.AuthTime {
		t.Errorf("auth_time is not match: sso_token=%d != code=%d", ssoToken.AuthTime, code.AuthTime)
	} else if code.Subject != ssoToken.Subject {
		t.Errorf("auth_time is not match: sso_token=%s != code=%s", ssoToken.Subject, code.Subject)
	}

	t.Log("---------- try login by another client with SSO token ----------")

	params = url.Values{
		"redirect_uri":  {"http://implicit-client.example.com/callback"},
		"client_id":     {"implicit_client_id"},
		"response_type": {"code"},
	}
	req, _ = http.NewRequest("GET", "/authz?"+params.Encode(), nil)
	for _, c := range rawCookie {
		req.Header.Add("Cookie", c)
	}

	resp = env.DoRequest(req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code on another client with SSO token: %d", resp.Code)
	}

	inputs, err = testutil.FindInputsByHTML(resp.Body)
	if err != nil {
		t.Fatalf("failed to parse consent page: %s", err)
	}

	if _, ok := inputs["username"]; ok {
		t.Errorf("expected consent page but got username input")
	}

	if _, ok := inputs["password"]; ok {
		t.Errorf("expected consent page but got password input")
	}

	params = url.Values{}
	for k, v := range inputs {
		params.Add(k, v)
	}

	req, _ = http.NewRequest("POST", "/authz", strings.NewReader(params.Encode()))
	for _, c := range rawCookie {
		req.Header.Add("Cookie", c)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp = env.DoRequest(req)
	if resp.Code != http.StatusFound {
		t.Fatalf("unexpected status code on login via consent prompt: %d", resp.Code)
	}

	location, err = url.Parse(resp.Header().Get("Location"))
	if err != nil {
		t.Errorf("failed to parse location: %s", err)
	}
	code, err = env.API.TokenManager.ParseCode(location.Query().Get("code"))
	if err != nil {
		t.Errorf("failed to parse code: %s", err)
	} else if err = code.Validate(env.API.Config.Issuer); err != nil {
		t.Errorf("respond code is invalid: %s", err)
	} else if code.AuthTime != ssoToken.AuthTime {
		t.Errorf("auth_time is not match: sso_token=%d != code=%d", ssoToken.AuthTime, code.AuthTime)
	} else if code.Subject != ssoToken.Subject {
		t.Errorf("auth_time is not match: sso_token=%s != code=%s", ssoToken.Subject, code.Subject)
	}

	t.Log("---------- first client still can login with SSO token ----------")

	params = url.Values{
		"redirect_uri":  {"http://some-client.example.com/callback"},
		"client_id":     {"some_client_id"},
		"response_type": {"code"},
	}
	req, _ = http.NewRequest("GET", "/authz?"+params.Encode(), nil)
	for _, c := range rawCookie {
		req.Header.Add("Cookie", c)
	}

	resp = env.DoRequest(req)
	if resp.Code != http.StatusFound {
		t.Fatalf("unexpected status code on login with SSO token: %d", resp.Code)
	}

	location, err = url.Parse(resp.Header().Get("Location"))
	if err != nil {
		t.Errorf("failed to parse location: %s", err)
	}
	code, err = env.API.TokenManager.ParseCode(location.Query().Get("code"))
	if err != nil {
		t.Errorf("failed to parse code: %s", err)
	} else if err = code.Validate(env.API.Config.Issuer); err != nil {
		t.Errorf("respond code is invalid: %s", err)
	} else if code.AuthTime != ssoToken.AuthTime {
		t.Errorf("auth_time is not match: sso_token=%d != code=%d", ssoToken.AuthTime, code.AuthTime)
	} else if code.Subject != ssoToken.Subject {
		t.Errorf("auth_time is not match: sso_token=%s != code=%s", ssoToken.Subject, code.Subject)
	}
}

func TestUseLoginSession(t *testing.T) {
	env := testutil.NewAPITestEnvironment(t)

	params := url.Values{
		"redirect_uri":  {"http://some-client.example.com/callback"},
		"client_id":     {"some_client_id"},
		"response_type": {"code"},
	}

	resp := env.Get("/authz", "", params)
	if resp.Code != http.StatusOK {
		t.Fatalf("failed to get login form (status code = %d)", resp.Code)
	}

	inputs, err := testutil.FindInputsByHTML(resp.Body)
	if err != nil {
		t.Fatalf("failed to parse login form: %s", err)
	}
	t.Logf("session token is %#v", inputs["session"])

	params.Set("username", "macrat")
	params.Set("password", "foobar")
	params.Set("session", inputs["session"])

	resp = env.Post("/authz", "", params)
	if resp.Code != http.StatusFound {
		t.Fatalf("failed to get login form (status code = %d)", resp.Code)
	}

	location, err := url.Parse(resp.Header().Get("Location"))
	if err != nil {
		t.Errorf("failed to parse location: %s", err)
	}
	if errMsg := location.Query().Get("error"); errMsg != "" {
		t.Errorf("redirect location includes error message: %s", errMsg)
	}
}

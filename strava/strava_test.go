package strava

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeURL(t *testing.T) {
	u := AuthorizeURL("12345", "http://localhost:8787/callback")
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "www.strava.com", parsed.Host)
	assert.Equal(t, "/oauth/authorize", parsed.Path)
	q := parsed.Query()
	assert.Equal(t, "12345", q.Get("client_id"))
	assert.Equal(t, "http://localhost:8787/callback", q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "activity:read_all", q.Get("scope"))
}

func TestExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "secret", r.Form.Get("client_secret"))
		assert.Equal(t, "the-code", r.Form.Get("code"))
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","expires_at":2000000000}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	tokenEndpoint = server.URL

	token, err := ExchangeCode("12345", "secret", "the-code", "http://localhost:8787/callback")
	require.NoError(t, err)
	assert.Equal(t, "access-1", token.AccessToken)
	assert.Equal(t, "refresh-1", token.RefreshToken)
	assert.Equal(t, time.Unix(2000000000, 0), token.ExpiresAt)
}

func TestRequestTokenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
	}))
	defer server.Close()
	tokenEndpoint = server.URL

	_, err := ExchangeCode("12345", "secret", "the-code", "http://localhost:8787/callback")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestRefreshIfNeeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		require.NoError(t, err)
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "old-refresh", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_at":2000000000}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	tokenEndpoint = server.URL

	client := NewClient("12345", "secret", Token{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})
	refreshed, err := client.RefreshIfNeeded()
	require.NoError(t, err)
	assert.True(t, refreshed)
	assert.Equal(t, "new-access", client.Tokens().AccessToken)
	assert.Equal(t, "new-refresh", client.Tokens().RefreshToken)
}

func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fresh-access", r.Header.Get("Authorization"))
		assert.Equal(t, "/athlete", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"firstname":"Martin","lastname":"Lehoux"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	apiEndpoint = server.URL

	client := NewClient("12345", "secret", Token{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	var athlete struct {
		Firstname string `json:"firstname"`
	}
	err := client.GetJSON("/athlete", &athlete)
	require.NoError(t, err)
	assert.Equal(t, "Martin", athlete.Firstname)
}

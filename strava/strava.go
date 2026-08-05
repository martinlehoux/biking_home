package strava

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
)

const (
	AuthURL  = "https://www.strava.com/oauth/authorize"
	TokenURL = "https://www.strava.com/oauth/token"
	APIURL   = "https://www.strava.com/api/v3"
)

var (
	tokenEndpoint = TokenURL
	apiEndpoint   = APIURL
)

type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func AuthorizeURLWithState(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("approval_prompt", "auto")
	q.Set("scope", "activity:read_all")
	if state != "" {
		q.Set("state", state)
	}
	return AuthURL + "?" + q.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

func ExchangeCode(clientID, clientSecret, code, redirectURI string) (Token, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	return requestToken(form)
}

func Refresh(clientID, clientSecret, refreshToken string) (Token, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	return requestToken(form)
}

func requestToken(form url.Values) (Token, error) {
	resp, err := http.PostForm(tokenEndpoint, form)
	if err != nil {
		return Token{}, kcore.Wrap(err, "failed to request Strava token")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, kcore.Wrap(err, "failed to read Strava token response")
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("strava token request failed (HTTP %d): %s", resp.StatusCode, body)
	}
	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Token{}, kcore.Wrap(err, "failed to decode Strava token response")
	}
	if parsed.AccessToken == "" {
		return Token{}, fmt.Errorf("strava token response missing access_token: %s", body)
	}
	return Token{
		AccessToken:  parsed.AccessToken,
		RefreshToken: parsed.RefreshToken,
		ExpiresAt:    time.Unix(parsed.ExpiresAt, 0),
	}, nil
}

type Client struct {
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
	tokens       Token
}

func NewClient(clientID, clientSecret string, tokens Token) *Client {
	return &Client{ClientID: clientID, ClientSecret: clientSecret, HTTP: http.DefaultClient, tokens: tokens}
}

func (c *Client) Tokens() Token {
	return c.tokens
}

func (c *Client) RefreshIfNeeded() (bool, error) {
	if time.Until(c.tokens.ExpiresAt) > 5*time.Minute {
		return false, nil
	}
	refreshed, err := Refresh(c.ClientID, c.ClientSecret, c.tokens.RefreshToken)
	if err != nil {
		return false, kcore.Wrap(err, "failed to refresh Strava access token")
	}
	c.tokens = refreshed
	return true, nil
}

func (c *Client) GetJSON(path string, out any) error {
	if _, err := c.RefreshIfNeeded(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, apiEndpoint+path, nil)
	if err != nil {
		return kcore.Wrap(err, "failed to build Strava request")
	}
	req.Header.Set("Authorization", "Bearer "+c.tokens.AccessToken)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return kcore.Wrap(err, "failed to call Strava API")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return kcore.Wrap(err, "failed to read Strava API response")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("strava API %s failed (HTTP %d): %s", path, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return kcore.Wrap(err, "failed to decode Strava API response")
	}
	return nil
}

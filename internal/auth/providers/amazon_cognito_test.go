package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/buzzfeed/sso/internal/pkg/sessions"
	"github.com/buzzfeed/sso/internal/pkg/testutil"
)

func newAmazonCognitoProviderServer(body []byte, code int) (*url.URL, *httptest.Server) {
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(code)
		rw.Write(body)
	}))
	u, _ := url.Parse(s.URL)
	return u, s
}
func newAmazonCognitoProvider(providerData *ProviderData, t *testing.T) *AmazonCognitoProvider {
	if providerData == nil {
		providerData = &ProviderData{
			ProviderName: "",
			ClientID:     "",
			ClientSecret: "",
			SignInURL:    &url.URL{},
			RedeemURL:    &url.URL{},
			RevokeURL:    &url.URL{},
			ProfileURL:   &url.URL{},
			ValidateURL:  &url.URL{},
			Scope:        ""}
	}
	provider, err := NewAmazonCognitoProvider(providerData, "test.amazonCognito.com")
	if err != nil {
		t.Fatalf("new amazonCognito provider returns unexpected error: %q", err)
	}
	return provider
}

func TestAmazonCognitoProviderDefaults(t *testing.T) {
	expectedResults := []struct {
		name         string
		providerData *ProviderData
		signInURL    string
		redeemURL    string
		revokeURL    string
		profileURL   string
		userInfoURL  string
		validateURL  string
		scope        string
	}{
		{
			name:        "defaults",
			signInURL:   "https://test.amazonCognito.com/oauth2/authorize",
			redeemURL:   "https://test.amazonCognito.com/oauth2/token",
			revokeURL:   "", // TODO
			profileURL:  "https://test.amazonCognito.com/oauth2/userInfo",
			validateURL: "https://test.amazonCognito.com/oauth2/userInfo",
			scope:       "openid profile email aws.cognito.signin.user.admin",
		},
	}
	for _, expected := range expectedResults {
		t.Run(expected.name, func(t *testing.T) {
			p := newAmazonCognitoProvider(expected.providerData, t)
			if p == nil {
				t.Errorf("AmazonCognito provider was nil")
			}
			if p.Data().ProviderName != "AmazonCognito" {
				t.Errorf("expected provider name AmazonCognito, got %q", p.Data().ProviderName)
			}
			if p.Data().SignInURL.String() != expected.signInURL {
				t.Logf("expected %q", expected.signInURL)
				t.Logf("     got %q", p.Data().SignInURL.String())
				t.Errorf("unexpected signin url")
			}
			if p.Data().RedeemURL.String() != expected.redeemURL {
				t.Logf("expected %q", expected.redeemURL)
				t.Logf("     got %q", p.Data().RedeemURL.String())
				t.Errorf("unexpected redeem url")
			}

			if p.Data().RevokeURL.String() != expected.revokeURL {
				t.Logf("expected %q", expected.revokeURL)
				t.Logf("     got %q", p.Data().RevokeURL.String())
				t.Errorf("unexpected revoke url")
			}

			if p.Data().ValidateURL.String() != expected.validateURL {
				t.Logf("expected %q", expected.validateURL)
				t.Logf("     got %q", p.Data().ValidateURL.String())
				t.Errorf("unexpected validate url")
			}

			if p.Data().ProfileURL.String() != expected.profileURL {
				t.Logf("expected %q", expected.profileURL)
				t.Logf("     got %q", p.Data().ProfileURL.String())
				t.Errorf("unexpected profile url")
			}

			if p.Data().Scope != expected.scope {
				t.Logf("expected %q", expected.scope)
				t.Logf("     got %q", p.Data().Scope)
				t.Errorf("unexpected scope")
			}
		})

	}
}

type amazonCognitoProviderRedeemResponse struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	EmailAddress  string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	ExpiresIn     int64  `json:"expires_in"`
}

func TestAmazonCognitoProviderRedeem(t *testing.T) {
	testCases := []struct {
		name            string
		resp            amazonCognitoProviderRedeemResponse
		expectedError   bool
		expectedSession *sessions.SessionState
	}{
		{
			name: "redeem",
			resp: amazonCognitoProviderRedeemResponse{
				AccessToken:   "a1234",
				EmailAddress:  "michael.bland@gsa.gov",
				EmailVerified: true,
				ExpiresIn:     10,
				RefreshToken:  "refresh12345",
			},
			expectedSession: &sessions.SessionState{
				Email:        "michael.bland@gsa.gov",
				AccessToken:  "a1234",
				RefreshToken: "refresh12345",
			},
		},
		{
			name: "missing email",
			resp: amazonCognitoProviderRedeemResponse{
				AccessToken: "a1234",
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := newAmazonCognitoProvider(nil, t)
			body, err := json.Marshal(tc.resp)
			testutil.Equal(t, nil, err)
			var server *httptest.Server
			p.RedeemURL, server = newAmazonCognitoProviderServer(body, http.StatusOK)
			p.ProfileURL, server = newAmazonCognitoProviderServer(body, http.StatusOK)
			defer server.Close()

			session, err := p.Redeem("http://redirect/", "code1234")
			if tc.expectedError && err == nil {
				t.Errorf("expected redeem error but was nil")
			}
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error %q", err)
			}
			if tc.expectedSession == nil && session != nil {
				t.Errorf("expected session to be nil but it was %q", session)
			}
			if session != nil && tc.expectedSession != nil {
				if session.Email != tc.expectedSession.Email {
					t.Logf("expected email %q", tc.expectedSession.Email)
					t.Logf("           got %q", session.Email)
					t.Errorf("unexpected session email")
				}

				if session.AccessToken != tc.expectedSession.AccessToken {
					t.Logf("expected access token %q", tc.expectedSession.AccessToken)
					t.Logf("                  got %q", session.AccessToken)
					t.Errorf("unexpected access token")
				}

				if session.RefreshToken != tc.expectedSession.RefreshToken {
					t.Logf("expected refresh token %q", tc.expectedSession.RefreshToken)
					t.Logf("                   got %q", session.RefreshToken)
					t.Errorf("unexpected session refresh token")
				}
			}
		})
	}
}

type amazonCognitoProviderRevokeErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func TestAmazonCognitoProviderRevoke(t *testing.T) {
	testCases := []struct {
		name                string
		resp                amazonCognitoProviderRevokeErrorResponse
		httpStatus          int
		sessionState        *sessions.SessionState
		expectedError       bool
		expectedErrorString string
	}{
		{
			name: "idempotent revoke",
			resp: amazonCognitoProviderRevokeErrorResponse{
				Error:            "invalid_token",
				ErrorDescription: "Token expired or revoked",
			},
			sessionState: &sessions.SessionState{
				AccessToken:     "access1234",
				RefreshDeadline: time.Now(),
				RefreshToken:    "refresh1234",
				Email:           "logged.out@example.com",
			},
			httpStatus: http.StatusBadRequest,
		},
		{
			name: "can still fail",
			resp: amazonCognitoProviderRevokeErrorResponse{
				Error:            "not_invalid_token",
				ErrorDescription: "Something else happened internally",
			},
			sessionState: &sessions.SessionState{
				AccessToken:     "access1234",
				RefreshDeadline: time.Now(),
				RefreshToken:    "refresh1234",
				Email:           "logged.out@example.com",
			},
			httpStatus:          http.StatusForbidden,
			expectedErrorString: "SERVICE_UNAVAILABLE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := newAmazonCognitoProvider(nil, t)
			body, err := json.Marshal(tc.resp)
			testutil.Equal(t, nil, err)
			var server *httptest.Server
			p.RevokeURL, server = newAmazonCognitoProviderServer(body, tc.httpStatus)
			defer server.Close()

			err = p.Revoke(tc.sessionState)
			if tc.expectedError && err != nil {
				t.Errorf("unexpected error %q", err)
			}
			if tc.expectedError {
				if err == nil {
					t.Errorf("expected error but err was nil")
				}
				if !strings.Contains(err.Error(), tc.expectedErrorString) {
					t.Logf("expected error string to contain %q", tc.expectedErrorString)
					t.Logf("                             got %q", err)
					t.Errorf("unexpected error string")
				}
			}
		})

	}
}

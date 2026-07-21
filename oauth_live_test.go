// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

//go:build integration

package ynab_test

// The OAuth-only live probes — the three truths a personal access token
// cannot reach: the PlanIDDefault positive path (default-plan selection
// happens during OAuth consent), the scope=read-only 403 on writes, and
// TokenSource per-attempt consultation with a genuinely refreshed
// access token. Runs with the YNAB_OAUTH_* environment set (client
// credentials plus refresh tokens obtained once via the authorization
// code grant); self-skips without them. Token-endpoint calls go to
// app.ynab.com and do not spend the api.ynab.com request quota.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.venceslau.dev/ynab"
)

// oauthEndpointURL is YNAB's token-exchange endpoint (documented at
// api.ynab.com).
const oauthEndpointURL = "https://app.ynab.com/oauth/token"

// safeTokenRe bounds a refresh token to characters safe in a sourced
// KEY=value file — YNAB's are URL-safe base64ish; anything else is
// refused rather than persisted.
var safeTokenRe = regexp.MustCompile(`^[A-Za-z0-9._~/+=-]+$`)

// refreshingTokenSource is the consumer-side OAuth seam: it exchanges a
// refresh token for access tokens on demand and PERSISTS the newest
// refresh token — YNAB invalidates ancestors once the chain advances
// (probed live 2026-07-20; see API_NOTES.md), so an unpersisted
// rotation strands the stored credential. The library is deliberately
// not an OAuth flow — this is what a real consumer writes.
type refreshingTokenSource struct {
	clientID     string
	clientSecret string
	// persistKey/persistFile: the KEY=value line and file each rotation
	// rewrites. Two sources may share one file; persist()'s
	// read-modify-write is safe only because the suite is sequential by
	// contract (paralleltest is exempted for the live files).
	persistKey  string
	persistFile string

	mu           sync.Mutex
	refreshToken string
	accessToken  string
	refreshes    int
	forceRefresh bool // test hook: the next Token call must hit the token endpoint
}

// persist rewrites persistKey's line in persistFile with the newest
// refresh token, so the on-disk credential always tracks the head of
// the rotation chain. Called under s.mu.
func (s *refreshingTokenSource) persist() error {
	if s.persistFile == "" {
		return nil
	}
	raw, err := os.ReadFile(s.persistFile)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	replaced := false
	for i, l := range lines {
		if strings.HasPrefix(l, s.persistKey+"=") {
			lines[i] = s.persistKey + "=" + s.refreshToken
			replaced = true
		}
	}
	if !replaced {
		lines = append(lines, s.persistKey+"="+s.refreshToken)
	}
	return os.WriteFile(s.persistFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// Token implements ynab.TokenSource; consulted before every attempt.
func (s *refreshingTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && !s.forceRefresh {
		return s.accessToken, nil
	}
	s.forceRefresh = false

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {s.refreshToken},
		"client_id":     {s.clientID},
		"client_secret": {s.clientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthEndpointURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth_live_test: token endpoint answered %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("oauth_live_test: decode token response: %w", err)
	}
	s.accessToken = tok.AccessToken
	if tok.RefreshToken != "" && tok.RefreshToken != s.refreshToken {
		// The rotated token is written to a KEY=value file the CI later
		// sources; refuse anything outside a safe charset so a hostile
		// token-endpoint response can never smuggle shell metacharacters
		// into a step holding the secrets PAT.
		if !safeTokenRe.MatchString(tok.RefreshToken) {
			return "", errors.New("oauth_live_test: refresh token has unexpected characters — refusing to persist")
		}
		s.refreshToken = tok.RefreshToken // rotated: ancestors die once this is used
		if err := s.persist(); err != nil {
			return "", fmt.Errorf("oauth_live_test: persist rotated refresh token: %w", err)
		}
	}
	s.refreshes++
	return s.accessToken, nil
}

// TestLiveOAuth runs the OAuth-only probes, sequentially like every live
// suite. Budget: ~8 api.ynab.com requests.
func TestLiveOAuth(t *testing.T) {
	clientID := os.Getenv("YNAB_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("YNAB_OAUTH_CLIENT_SECRET")
	refresh := os.Getenv("YNAB_OAUTH_REFRESH_TOKEN")
	if clientID == "" || clientSecret == "" || refresh == "" {
		skipOrFail(t, "YNAB_OAUTH_CLIENT_ID/SECRET/REFRESH_TOKEN not set — OAuth probes need a one-time consent grant")
	}

	persistFile := os.Getenv("YNAB_OAUTH_TOKEN_FILE") // empty disables persistence
	src := &refreshingTokenSource{
		clientID: clientID, clientSecret: clientSecret, refreshToken: refresh,
		persistKey: "REFRESH_TOKEN", persistFile: persistFile,
	}
	client := ynab.NewWithTokenSource(src)

	user, err := client.User(t.Context())
	require.NoError(t, err, "an OAuth bearer token must ride the same Authorization path as a PAT")
	require.NotEmpty(t, user.ID)

	// Per-attempt consultation with a genuinely NEW token: force a
	// refresh between two calls — the second must succeed on the rotated
	// access token, proving the seam end to end.
	src.mu.Lock()
	src.forceRefresh = true
	src.mu.Unlock()
	plans, err := client.Plans(t.Context())
	require.NoError(t, err, "the rotated access token must be picked up on the next attempt")
	require.NotEmpty(t, plans.Plans)
	require.NotNil(t, plans.DefaultPlan,
		"the consent selected a default plan — PlanList.default_plan must carry it under this grant")
	require.GreaterOrEqual(t, src.refreshes, 2, "the force-refresh hook must have hit the token endpoint again")

	// PlanIDDefault's positive path exists only under an OAuth grant
	// whose consent selected a default plan (ledger: 404.2 under a PAT).
	settings, err := client.Plan(ynab.PlanIDDefault).Settings(t.Context())
	if err != nil {
		require.ErrorIs(t, err, ynab.ErrResourceNotFound,
			"without a consent-selected default plan the sentinel must answer 404.2, like under a PAT")
		t.Log("PlanIDDefault: no default plan selected during consent — " +
			"404.2 branch (re-consent with selection to prove the positive path)")
	} else {
		require.NotNil(t, settings.CurrencyFormat)
		t.Log("PlanIDDefault: positive path PROVEN — consent-selected default plan resolved (update API_NOTES.md)")
	}

	// scope=read-only: a write must answer a real 403 through the
	// sentinel taxonomy. Requires the second, read-only grant.
	roRefresh := os.Getenv("YNAB_OAUTH_RO_REFRESH_TOKEN")
	if roRefresh == "" {
		skipOrFail(t, "YNAB_OAUTH_RO_REFRESH_TOKEN not set — the read-only-scope 403 probe needs the second grant")
		return
	}
	roSrc := &refreshingTokenSource{
		clientID: clientID, clientSecret: clientSecret, refreshToken: roRefresh,
		persistKey: "RO_REFRESH_TOKEN", persistFile: persistFile,
	}
	roClient := ynab.NewWithTokenSource(roSrc)

	planID := ynab.PlanIDLastUsed
	if id := os.Getenv("YNAB_TEST_PLAN_ID"); id != "" {
		planID = ynab.PlanID(id)
	}
	roPlan := roClient.Plan(planID)

	payees, _, err := roPlan.Payees.List(t.Context())
	require.NoError(t, err, "reads must work under the read-only scope")
	require.NotEmpty(t, payees)

	_, _, err = roPlan.Payees.Rename(t.Context(), payees[0].ID, payees[0].Name)
	require.ErrorIs(t, err, ynab.ErrForbidden,
		"a write under scope=read-only must answer 403 through the class sentinel")
	require.ErrorIs(t, err, ynab.ErrUnauthorizedScope,
		"the scope 403 is sub-code 403.3 (proven live 2026-07-20)")
	var apiErr *ynab.Error
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	require.Equal(t, "403.3", apiErr.ID)
}

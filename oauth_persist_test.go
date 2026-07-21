// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

//go:build integration

package ynab_test

// Tokenless unit coverage for the credential-persistence path — the
// costliest failure in the repo is a stranded refresh chain, so its
// mechanics are proven here at PR time, never only live.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefreshTokenPersist(t *testing.T) {
	t.Parallel()

	t.Run("rewrites the keyed line, leaves the rest untouched", func(t *testing.T) {
		t.Parallel()
		f := filepath.Join(t.TempDir(), "creds")
		require.NoError(t, os.WriteFile(f,
			[]byte("CLIENT_ID=abc\nREFRESH_TOKEN=old\nRO_REFRESH_TOKEN=ro\n"), 0o600))

		src := &refreshingTokenSource{persistKey: "REFRESH_TOKEN", persistFile: f, refreshToken: "new-v2"}
		require.NoError(t, src.persist())

		require.Equal(t, "CLIENT_ID=abc\nREFRESH_TOKEN=new-v2\nRO_REFRESH_TOKEN=ro\n", readFile(t, f))
		fi, err := os.Stat(f)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), "the rewritten file must stay 0600")
	})

	t.Run("appends when the key is absent", func(t *testing.T) {
		t.Parallel()
		f := filepath.Join(t.TempDir(), "creds")
		require.NoError(t, os.WriteFile(f, []byte("CLIENT_ID=abc\n"), 0o600))

		src := &refreshingTokenSource{persistKey: "REFRESH_TOKEN", persistFile: f, refreshToken: "v1"}
		require.NoError(t, src.persist())
		require.Equal(t, "CLIENT_ID=abc\nREFRESH_TOKEN=v1\n", readFile(t, f))
	})

	t.Run("is atomic: no temp file survives a successful write", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f := filepath.Join(dir, "creds")
		require.NoError(t, os.WriteFile(f, []byte("REFRESH_TOKEN=old\n"), 0o600))

		src := &refreshingTokenSource{persistKey: "REFRESH_TOKEN", persistFile: f, refreshToken: "new"}
		require.NoError(t, src.persist())

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1, "the .tmp sibling must be renamed away, never left behind")
		require.Equal(t, "creds", entries[0].Name())
	})

	t.Run("empty persistFile is a no-op", func(t *testing.T) {
		t.Parallel()
		src := &refreshingTokenSource{persistKey: "REFRESH_TOKEN", refreshToken: "x"}
		require.NoError(t, src.persist())
	})
}

func TestSafeTokenRe(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"aBc123", "a.b_c~d", "a/b+c=d-e", "eyJ0eXAiOiJKV1Qi"} {
		require.True(t, safeTokenRe.MatchString(ok), "%q must be accepted", ok)
	}
	for _, bad := range []string{"", "a b", "a;rm -rf", "a$(x)", "a`x`", "a\nb", "a\"b"} {
		require.False(t, safeTokenRe.MatchString(bad), "%q must be refused — it would reach a sourced file", bad)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package ynab_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"pkg.venceslau.dev/ynab"
)

// ExampleWithBaseURL is the first-class mocking seam: point the client at
// an httptest.Server. Nothing from this library's internals is needed —
// stdlib plus the public constructor is the whole story.
func ExampleWithBaseURL() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"u-123"}}}`))
	}))
	defer srv.Close()

	client := ynab.New("test-token", ynab.WithBaseURL(srv.URL))
	user, err := client.User(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("user:", user.ID)

	// Output:
	// user: u-123
}

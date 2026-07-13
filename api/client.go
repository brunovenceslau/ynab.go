// Copyright (c) 2018, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

// Package api implements shared structures and behaviours of
// the API services
package api // import "pkg.venceslau.dev/ynab/api"

// ClientReader contract for a read only client
type ClientReader interface {
	GET(url string, responseModel interface{}) error
}

// ClientWriter contract for a write only client
type ClientWriter interface {
	POST(url string, responseModel interface{}, requestBody []byte) error
	PUT(url string, responseModel interface{}, requestBody []byte) error
	PATCH(url string, responseModel interface{}, requestBody []byte) error
	DELETE(url string, responseModel interface{}) error
}

// ClientReaderWriter contract for a read-write client
type ClientReaderWriter interface {
	ClientReader
	ClientWriter
}

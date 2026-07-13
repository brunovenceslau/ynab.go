// Copyright (c) 2019, Bruno M V Souza <github@b.bmvs.io>. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause license that can be
// found in the LICENSE file.

package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"pkg.venceslau.dev/ynab/api"
)

func TestFilter_ToQuery(t *testing.T) {
	table := []struct {
		Input  api.Filter
		Output string
	}{
		{
			Input:  api.Filter{LastKnowledgeOfServer: 2},
			Output: "last_knowledge_of_server=2",
		},
		{
			Input:  api.Filter{LastKnowledgeOfServer: 0},
			Output: "last_knowledge_of_server=0",
		},
		{
			Input:  api.Filter{},
			Output: "last_knowledge_of_server=0",
		},
	}

	for _, test := range table {
		assert.Equal(t, test.Output, test.Input.ToQuery())
	}
}

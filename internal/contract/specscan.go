// Copyright 2026 Bruno Venceslau. All rights reserved.
// Use of this source code is governed by a BSD-2-Clause
// license that can be found in the LICENSE file.

package contract

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SpecOp is one operation tuple extracted from the vendored spec.
type SpecOp struct {
	ID          string
	Method      string
	Path        string
	QueryParams []string
	// HasBody reports whether the operation declares a requestBody —
	// the wire truth DiffSpec checks bodilessOps against.
	HasBody bool
}

// Spec is the scanner's view of the vendored openapi.yaml.
type Spec struct {
	Version string
	Ops     []SpecOp
}

// Line shapes of the spec's regular paths: layout. Query parameters are
// declared inline under each operation (never $ref), so a line scanner is
// sufficient; the deliberate-mutation tests keep it honest. If the spec's
// layout ever stops being this regular, replacing this with a YAML parser
// is an ask-first dependency decision.
var (
	rePathLine    = regexp.MustCompile(`^  (/[^:]*):\s*$`)
	reVerbLine    = regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
	reOperationID = regexp.MustCompile(`^      operationId:\s*(\S+)`)
	reParamName   = regexp.MustCompile(`^\s+- name:\s*(\S+)`)
	reParamIn     = regexp.MustCompile(`^\s+in:\s*(\S+)`)
	reVersion     = regexp.MustCompile(`^  version:\s*(\S+)`)
	reRequestBody = regexp.MustCompile(`^      requestBody:\s*$`)
	reResponses   = regexp.MustCompile(`^      responses:\s*$`)
)

// ScanSpec extracts {operationId, verb, path, query-param names} tuples and
// the info.version from the vendored spec file.
func ScanSpec(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("contract: read spec: %w", err)
	}

	spec := &Spec{}
	lines := strings.Split(string(raw), "\n")

	var curPath, curVerb string
	var curOp *SpecOp
	for i, line := range lines {
		switch {
		case rePathLine.MatchString(line):
			curPath = rePathLine.FindStringSubmatch(line)[1]
			curOp = nil
		case reVerbLine.MatchString(line):
			curVerb = strings.ToUpper(reVerbLine.FindStringSubmatch(line)[1])
			curOp = nil
		case reOperationID.MatchString(line):
			id := reOperationID.FindStringSubmatch(line)[1]
			spec.Ops = append(spec.Ops, SpecOp{ID: id, Method: curVerb, Path: curPath})
			curOp = &spec.Ops[len(spec.Ops)-1]
		case spec.Version == "" && reVersion.MatchString(line):
			spec.Version = reVersion.FindStringSubmatch(line)[1]
		default:
			curOp = scanOpLine(curOp, lines, i)
		}
	}

	if len(spec.Ops) == 0 {
		return nil, fmt.Errorf("contract: no operations found in %s — scanner or spec layout broken", path)
	}
	return spec, nil
}

// scanOpLine handles the lines belonging to an open operation:
// requestBody presence, query parameters, and the responses line that
// closes the op — parameters precede responses in every operation, so
// closing there keeps name/in-shaped lines inside response schemas from
// being misattributed.
func scanOpLine(curOp *SpecOp, lines []string, i int) *SpecOp {
	line := lines[i]
	switch {
	case reResponses.MatchString(line):
		return nil
	case curOp == nil:
		return nil
	case reRequestBody.MatchString(line):
		// requestBody sits between operationId and responses at the
		// operation-child indent, so the open op owns it.
		curOp.HasBody = true
	case reParamName.MatchString(line):
		name := reParamName.FindStringSubmatch(line)[1]
		if paramIsQuery(lines, i+1) {
			curOp.QueryParams = append(curOp.QueryParams, name)
		}
	}
	return curOp
}

// paramIsQuery looks at the lines following a "- name:" for the parameter's
// "in:" attribute, stopping at the next parameter.
func paramIsQuery(lines []string, from int) bool {
	for i := from; i < len(lines) && i < from+5; i++ {
		if reParamName.MatchString(lines[i]) {
			return false
		}
		if m := reParamIn.FindStringSubmatch(lines[i]); m != nil {
			return m[1] == "query"
		}
	}
	return false
}

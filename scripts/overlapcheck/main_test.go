// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		input   string
		want    int
		wantErr string
	}{
		{name: "disjoint", input: "10.0.0.0/24\n10.0.1.0/24\n\n10.0.2.0/25\n2001:db8::/64\n", want: 0},
		{name: "empty input", input: "", want: 0},
		{name: "overlapping", input: "10.0.0.0/24\n10.0.0.128/25\n", want: 1, wantErr: "10.0.0.0/24 overlaps 10.0.0.128/25"},
		{name: "overlap far apart in input", input: "10.0.5.0/24\n192.168.0.0/24\n10.0.0.0/16\n", want: 1, wantErr: "overlaps"},
		{name: "duplicate", input: "10.0.0.0/24\n10.0.1.0/24\n10.0.0.0/24\n", want: 1, wantErr: "duplicate CIDR 10.0.0.0/24"},
		{name: "within ok", args: []string{"-within", "10.0.0.0/18"}, input: "10.0.0.0/24\n10.0.63.0/24\n", want: 0},
		{name: "outside", args: []string{"-within", "10.0.0.0/18"}, input: "10.0.64.0/24\n", want: 1, wantErr: "outside"},
		{name: "wider than bound", args: []string{"-within", "10.0.0.0/18"}, input: "10.0.0.0/16\n", want: 1, wantErr: "outside"},
		{name: "count match", args: []string{"-count", "2"}, input: "10.0.0.0/24\n10.0.1.0/24\n", want: 0},
		{name: "count mismatch", args: []string{"-count", "3"}, input: "10.0.0.0/24\n10.0.1.0/24\n", want: 1, wantErr: "want exactly 3"},
		{name: "garbage", input: "not-a-cidr\n", want: 2, wantErr: "invalid CIDR"},
		{name: "host bits", input: "10.0.0.1/24\n", want: 2, wantErr: "host bits"},
		{name: "positional arg", args: []string{"x"}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			got := run(tt.args, strings.NewReader(tt.input), &stdout, &stderr)
			if got != tt.want {
				t.Fatalf("exit = %d, want %d (stderr %q)", got, tt.want, stderr.String())
			}

			if tt.wantErr != "" && !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr %q does not contain %q", stderr.String(), tt.wantErr)
			}
		})
	}
}

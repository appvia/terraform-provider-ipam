// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

// Command overlapcheck reads CIDRs, one per line, from stdin and exits 0 only
// when they are all valid, distinct and pairwise non-overlapping. It uses
// net/netip alone, so it does not trust the ipam library's own validation.
//
//	overlapcheck [-within CIDR]... [-count N] < cidrs.txt
//
// Exit codes: 0 disjoint, 1 overlap/duplicate/outside/count mismatch, 2 usage
// or unparsable input.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"sort"
	"strings"
)

type cidrList []netip.Prefix

func (l *cidrList) String() string { return fmt.Sprint(*l) }

func (l *cidrList) Set(v string) error {
	p, err := parsePrefix(v)
	if err != nil {
		return err
	}

	*l = append(*l, p)

	return nil
}

func parsePrefix(s string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid CIDR %q: %w", s, err)
	}

	if p != p.Masked() {
		return netip.Prefix{}, fmt.Errorf("invalid CIDR %q: host bits set", s)
	}

	return p, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("overlapcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var within cidrList

	fs.Var(&within, "within", "every CIDR must lie inside one of these (repeatable)")
	count := fs.Int("count", -1, "require exactly this many CIDRs")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "overlapcheck: unexpected arguments; CIDRs are read from stdin")
		return 2
	}

	var prefixes []netip.Prefix

	sc := bufio.NewScanner(stdin)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		p, err := parsePrefix(line)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "overlapcheck: %v\n", err)
			return 2
		}

		prefixes = append(prefixes, p)
	}

	if err := sc.Err(); err != nil {
		_, _ = fmt.Fprintf(stderr, "overlapcheck: read: %v\n", err)
		return 2
	}

	problems := check(prefixes, within, *count)
	for _, msg := range problems {
		_, _ = fmt.Fprintf(stderr, "overlapcheck: %s\n", msg)
	}

	if len(problems) > 0 {
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "overlapcheck: %d CIDRs, distinct and non-overlapping\n", len(prefixes))

	return 0
}

func check(prefixes []netip.Prefix, within []netip.Prefix, count int) []string {
	var problems []string

	if count >= 0 && len(prefixes) != count {
		problems = append(problems, fmt.Sprintf("got %d CIDRs, want exactly %d", len(prefixes), count))
	}

	sorted := append([]netip.Prefix(nil), prefixes...)
	sort.Slice(sorted, func(i, j int) bool {
		if c := sorted[i].Addr().Compare(sorted[j].Addr()); c != 0 {
			return c < 0
		}

		return sorted[i].Bits() < sorted[j].Bits()
	})

	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			a, b := sorted[i], sorted[j]

			switch {
			case a == b:
				problems = append(problems, "duplicate CIDR "+a.String())
			case a.Overlaps(b):
				problems = append(problems, fmt.Sprintf("%s overlaps %s", a, b))
			}
		}
	}

	if len(within) > 0 {
		for _, p := range prefixes {
			if !inside(p, within) {
				problems = append(problems, fmt.Sprintf("%s is outside %v", p, within))
			}
		}
	}

	return problems
}

func inside(p netip.Prefix, bounds []netip.Prefix) bool {
	for _, b := range bounds {
		if b.Addr().BitLen() == p.Addr().BitLen() && b.Bits() <= p.Bits() && b.Contains(p.Addr()) {
			return true
		}
	}

	return false
}

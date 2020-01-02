// Copyright 2018 The go-voc-core Authors
// This file is part of the go-voc-core library.
//
// The go-voc-core library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-voc-core library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-voc-core library. If not, see <http://www.gnu.org/licenses/>.

package accounts

import (
	"testing"
)

func TestURLParsing(t *testing.T) {
	url, err := parseURL("https://voc-core.org")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if url.Scheme != "https" {
		t.Errorf("expected: %v, got: %v", "https", url.Scheme)
	}
	if url.Path != "voc-core.org" {
		t.Errorf("expected: %v, got: %v", "voc-core.org", url.Path)
	}

	_, err = parseURL("voc-core.org")
	if err == nil {
		t.Error("expected err, got: nil")
	}
}

func TestURLString(t *testing.T) {
	url := URL{Scheme: "https", Path: "voc-core.org"}
	if url.String() != "https://voc-core.org" {
		t.Errorf("expected: %v, got: %v", "https://voc-core.org", url.String())
	}

	url = URL{Scheme: "", Path: "voc-core.org"}
	if url.String() != "voc-core.org" {
		t.Errorf("expected: %v, got: %v", "voc-core.org", url.String())
	}
}

func TestURLMarshalJSON(t *testing.T) {
	url := URL{Scheme: "https", Path: "voc-core.org"}
	json, err := url.MarshalJSON()
	if err != nil {
		t.Errorf("unexpcted error: %v", err)
	}
	if string(json) != "\"https://voc-core.org\"" {
		t.Errorf("expected: %v, got: %v", "\"https://voc-core.org\"", string(json))
	}
}

func TestURLUnmarshalJSON(t *testing.T) {
	url := &URL{}
	err := url.UnmarshalJSON([]byte("\"https://voc-core.org\""))
	if err != nil {
		t.Errorf("unexpcted error: %v", err)
	}
	if url.Scheme != "https" {
		t.Errorf("expected: %v, got: %v", "https", url.Scheme)
	}
	if url.Path != "voc-core.org" {
		t.Errorf("expected: %v, got: %v", "https", url.Path)
	}
}

func TestURLComparison(t *testing.T) {
	tests := []struct {
		urlA   URL
		urlB   URL
		expect int
	}{
		{URL{"https", "voc-core.org"}, URL{"https", "voc-core.org"}, 0},
		{URL{"http", "voc-core.org"}, URL{"https", "voc-core.org"}, -1},
		{URL{"https", "voc-core.org/a"}, URL{"https", "voc-core.org"}, 1},
		{URL{"https", "abc.org"}, URL{"https", "voc-core.org"}, -1},
	}

	for i, tt := range tests {
		result := tt.urlA.Cmp(tt.urlB)
		if result != tt.expect {
			t.Errorf("test %d: cmp mismatch: expected: %d, got: %d", i, tt.expect, result)
		}
	}
}

// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package simple

import (
	"testing"

	"github.com/google/winops/winlog"
)

func TestMakeBookmark(t *testing.T) {
	tests := []struct {
		name     string
		bookmark string
	}{
		{
			name:     "empty",
			bookmark: "",
		},
		{
			name:     "invalid_xml_falls_back",
			bookmark: "invalid-xml",
		},
		{
			name:     "nul_byte_falls_back_without_panic",
			bookmark: "<BookmarkList>\x00</BookmarkList>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, err := makeBookmark(tc.bookmark)
			if err != nil {
				t.Fatalf("makeBookmark(%q) returned unexpected error: %v", tc.bookmark, err)
			}
			if h == 0 {
				t.Fatalf("makeBookmark(%q) returned zero handle", tc.bookmark)
			}
			if err := winlog.Close(h); err != nil {
				t.Errorf("winlog.Close() returned unexpected error: %v", err)
			}
		})
	}
}

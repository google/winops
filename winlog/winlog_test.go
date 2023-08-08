// Copyright 2023 Google LLC
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

package winlog

import (
	"syscall"
	"testing"
)

func TestUtf16ToString(t *testing.T) {
	tests := []struct {
		input []uint16
		size  int
		want  string
	}{
		{syscall.StringToUTF16("foo"), 3, syscall.UTF16ToString(syscall.StringToUTF16("foo"))},
		{syscall.StringToUTF16("北京商务中心区"), len(syscall.StringToUTF16("北京商务中心区")), syscall.UTF16ToString(syscall.StringToUTF16("北京商务中心区"))},
		{syscall.StringToUTF16("foo"), 2, "fo"},
		{syscall.StringToUTF16("foo"), 10, "foo"},
		{[]uint16{'f', 0x0, 0x0, 'o', 'o'}, 5, "f\x00\x00oo"},
		{[]uint16{'f', 0x0, 0x0}, 3, "f"},
	}
	for _, test := range tests {
		got := utf16ToString(test.input, test.size)
		if got != test.want {
			t.Errorf("utf16ToString(%q, %d) = %q, want %q", test.input, test.size, got, test.want)
		}
	}
}

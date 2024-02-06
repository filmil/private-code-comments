package pkg

import (
	"fmt"
	"testing"

	lsp "go.lsp.dev/protocol"
)

func TestRPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		URI      lsp.URI
		ws       string
		expected string
	}{
		{lsp.URI("file:///foobar/file.txt"), "file:///foobar", "/file.txt"},
		{lsp.URI("file:///foobar/baz/file.txt"), "file:///foobar", "/baz/file.txt"},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			a := RPath(test.ws, test.URI)
			if a != test.expected {
				t.Errorf("want: %v, got: %v", test.expected, a)
			}
		})
	}
}

func TestFindWorkspace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		w      []lsp.WorkspaceFolder
		f      lsp.URI
		ws, fn string
	}{
		{
			[]lsp.WorkspaceFolder{
				{URI: "file:///ws", Name: "ws"},
			},
			lsp.URI("file:///ws/file.txt"),
			"file:///ws", "/file.txt",
		},
		{
			[]lsp.WorkspaceFolder{
				{URI: "file:///ws", Name: "ws"},
				{URI: "file:///ws2", Name: "ws"},
			},
			lsp.URI("file:///ws2/file.txt"),
			"file:///ws2", "/file.txt",
		},
	}

	for i, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			aw, af := FindWorkspace(test.w, test.f)
			if aw != test.ws || af != test.fn {
				t.Errorf("want: (w:%v, f:%v), got: (w:%v, f:%v)", test.ws, test.fn, aw, af)
			}
		})
	}
}

package pkg

import lsp "go.lsp.dev/protocol"

type PccGet struct {
	File lsp.URI `json:"file"`
	Line uint32  `json:"line"`
}

type PccGetResp struct {
	Content string `json:"content"`
}

type PccSet struct {
	PccGet
	Content string `json:"content"`
}

type PccSetRes struct{}

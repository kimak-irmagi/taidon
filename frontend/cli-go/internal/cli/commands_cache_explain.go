package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

type CacheExplainPrepareSpec struct {
	Class string `json:"class,omitempty"`
	Kind  string `json:"kind"`
	Image string `json:"image,omitempty"`
}

type CacheExplainRefContext struct {
	Requested      string `json:"requested,omitempty"`
	ResolvedCommit string `json:"resolvedCommit,omitempty"`
	Mode           string `json:"mode,omitempty"`
}

type CacheExplainDecision struct {
	Signature       string `json:"signature,omitempty"`
	MatchedStateID  string `json:"matchedStateId,omitempty"`
	ResolvedImageID string `json:"resolvedImageId,omitempty"`
}

type CacheExplainInput struct {
	Path string `json:"path"`
	Hash string `json:"hash,omitempty"`
}

type CacheExplainResult struct {
	Decision   string                  `json:"decision"`
	ReasonCode string                  `json:"reasonCode,omitempty"`
	Prepare    CacheExplainPrepareSpec `json:"prepare"`
	RefContext *CacheExplainRefContext `json:"refContext,omitempty"`
	Cache      CacheExplainDecision    `json:"cache"`
	Inputs     []CacheExplainInput     `json:"inputs"`
}

func PrintCacheExplain(w io.Writer, result CacheExplainResult, output string) error {
	if output == "json" {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		_, err = w.Write(append(data, '\n'))
		return err
	}

	fmt.Fprintf(w, "decision: %s\n", result.Decision)
	if result.ReasonCode != "" {
		fmt.Fprintf(w, "reasonCode: %s\n", result.ReasonCode)
	}
	fmt.Fprintf(w, "prepare.kind: %s\n", result.Prepare.Kind)
	if result.Prepare.Image != "" {
		fmt.Fprintf(w, "prepare.image: %s\n", result.Prepare.Image)
	}
	if result.Cache.Signature != "" {
		fmt.Fprintf(w, "cache.signature: %s\n", result.Cache.Signature)
	}
	if result.Cache.MatchedStateID != "" {
		fmt.Fprintf(w, "cache.stateId: %s\n", result.Cache.MatchedStateID)
	}
	if result.Cache.ResolvedImageID != "" {
		fmt.Fprintf(w, "cache.resolvedImageId: %s\n", result.Cache.ResolvedImageID)
	}
	if result.RefContext != nil {
		if result.RefContext.Requested != "" {
			fmt.Fprintf(w, "ref.requested: %s\n", result.RefContext.Requested)
		}
		if result.RefContext.ResolvedCommit != "" {
			fmt.Fprintf(w, "ref.resolvedCommit: %s\n", result.RefContext.ResolvedCommit)
		}
		if result.RefContext.Mode != "" {
			fmt.Fprintf(w, "ref.mode: %s\n", result.RefContext.Mode)
		}
	}
	fmt.Fprintf(w, "input.count: %d\n", len(result.Inputs))
	for i, input := range result.Inputs {
		if input.Hash != "" {
			fmt.Fprintf(w, "input[%d]: %s %s\n", i, input.Path, input.Hash)
			continue
		}
		fmt.Fprintf(w, "input[%d]: %s\n", i, input.Path)
	}
	return nil
}

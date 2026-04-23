package prepare

import (
	"context"
	"fmt"
)

func (m *PrepareService) CacheExplain(ctx context.Context, req Request) (CacheExplainPrepareResult, error) {
	prepared, err := m.prepareRequest(req)
	if err != nil {
		return CacheExplainPrepareResult{}, err
	}
	if errResp := m.ensureResolvedImageID(ctx, "", &prepared, nil); errResp != nil {
		return CacheExplainPrepareResult{}, errorFromExplainResponse(errResp)
	}

	tasks, stateID, errResp := m.buildPlan(ctx, "", prepared)
	if errResp != nil {
		return CacheExplainPrepareResult{}, errorFromExplainResponse(errResp)
	}

	signature := ""
	if prepared.request.PrepareKind == "lb" {
		signature, errResp = m.computeJobSignatureFromPlan(prepared, tasks)
	} else {
		signature, errResp = m.computeJobSignature(prepared)
	}
	if errResp != nil {
		return CacheExplainPrepareResult{}, errorFromExplainResponse(errResp)
	}

	cached, err := m.isStateCached(stateID)
	if err != nil {
		return CacheExplainPrepareResult{}, err
	}

	result := CacheExplainPrepareResult{
		Decision:        "miss",
		ReasonCode:      "no_matching_state",
		Signature:       signature,
		ResolvedImageID: prepared.effectiveImageID(),
	}
	if cached {
		result.Decision = "hit"
		result.ReasonCode = "exact_state_match"
		result.MatchedStateID = stateID
	}
	return result, nil
}

func errorFromExplainResponse(resp *ErrorResponse) error {
	if resp == nil {
		return fmt.Errorf("internal error")
	}
	if resp.Code == "invalid_argument" {
		return ValidationError{
			Code:    resp.Code,
			Message: resp.Message,
			Details: resp.Details,
		}
	}
	if resp.Details != "" {
		return fmt.Errorf("%s: %s", resp.Message, resp.Details)
	}
	if resp.Message != "" {
		return fmt.Errorf("%s", resp.Message)
	}
	return fmt.Errorf("internal error")
}

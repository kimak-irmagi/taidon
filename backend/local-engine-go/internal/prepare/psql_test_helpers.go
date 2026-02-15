package prepare

import "testing"

func psqlOutputStateID(t *testing.T, mgr *PrepareService, prepared preparedRequest, input TaskInput) string {
	t.Helper()
	digest, err := computePsqlContentDigest(prepared.psqlInputs, prepared.psqlWorkDir)
	if err != nil {
		t.Fatalf("computePsqlContentDigest: %v", err)
	}
	taskHash := psqlTaskHash(prepared.request.PrepareKind, digest.hash, mgr.version)
	outputID, errResp := mgr.computeOutputStateID(input.Kind, input.ID, taskHash)
	if errResp != nil {
		t.Fatalf("computeOutputStateID: %+v", errResp)
	}
	return outputID
}

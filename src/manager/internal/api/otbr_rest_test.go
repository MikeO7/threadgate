package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

func TestHandleCoprocessorVersion(t *testing.T) {
	tracker := runtime.NewTracker()
	tracker.UpdateRadioHealth("/dev/ttyUSB0", "RCP/2.0.0-test", "", "")
	server := NewServerWithThread(8081, thread.NewClient(thread.NewMock(), thread.PolicyBestEffort), true, "", tracker)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/node/coprocessor/version", nil)
	rr := httptest.NewRecorder()
	server.otbr.HandleCoprocessorVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var got string
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != "RCP/2.0.0-test" {
		t.Fatalf("got %q want RCP/2.0.0-test", got)
	}
}

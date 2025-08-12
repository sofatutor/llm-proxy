package dispatcher

import "testing"

func TestPermanentBackendError_Error(t *testing.T) {
    e := &PermanentBackendError{Msg: "permanent failure"}
    if e.Error() != "permanent failure" {
        t.Fatalf("unexpected error string: %q", e.Error())
    }
}



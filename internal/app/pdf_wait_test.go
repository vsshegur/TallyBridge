package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPDFWaitsForLauncherChild(t *testing.T) {
	p := filepath.Join(t.TempDir(), "report.pdf")
	exit := make(chan error, 1)
	exit <- nil
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(30 * time.Millisecond)
		os.WriteFile(p, []byte("%PDF-1.7\npartial"), 0600)
		time.Sleep(30 * time.Millisecond)
		os.WriteFile(p, []byte("%PDF-1.7\ncomplete\n%%EOF\n"), 0600)
	}()
	data, e := waitForPDF(ctx, p, exit)
	<-done
	if e != nil || !completePDF(data) {
		t.Fatalf("child output lost: %v", e)
	}
}
func TestPDFExitWithoutFileIsNotSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	exit := make(chan error, 1)
	exit <- nil
	if _, e := waitForPDF(ctx, filepath.Join(t.TempDir(), "missing.pdf"), exit); e == nil {
		t.Fatal("missing PDF accepted")
	}
}
func TestPDFCompleteOutputSurvivesBrowserExitError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "report.pdf")
	os.WriteFile(p, []byte("%PDF-1.7\ncomplete\n%%EOF"), 0600)
	exit := make(chan error, 1)
	exit <- errors.New("shutdown failed")
	if _, e := waitForPDF(context.Background(), p, exit); e != nil {
		t.Fatal(e)
	}
}

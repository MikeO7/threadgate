package main

import (
	"errors"
	"testing"
)

type stubApp struct {
	err error
}

func (s stubApp) Run() error { return s.err }

func TestRunSuccess(t *testing.T) {
	old := newAppRunner
	newAppRunner = func() appRunner { return stubApp{} }
	t.Cleanup(func() { newAppRunner = old })

	if err := run(); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
}

func TestRunFailure(t *testing.T) {
	old := newAppRunner
	newAppRunner = func() appRunner { return stubApp{err: errors.New("boot failed")} }
	t.Cleanup(func() { newAppRunner = old })

	if err := run(); err == nil {
		t.Fatal("expected run() error")
	}
}

func TestMainSuccess(t *testing.T) {
	oldRunner := newAppRunner
	oldExit := exitFunc
	newAppRunner = func() appRunner { return stubApp{} }
	exitFunc = func(code int) { t.Fatalf("unexpected exit %d", code) }
	t.Cleanup(func() {
		newAppRunner = oldRunner
		exitFunc = oldExit
	})
	main()
}

func TestMainFailure(t *testing.T) {
	oldRunner := newAppRunner
	oldExit := exitFunc
	exited := false
	newAppRunner = func() appRunner { return stubApp{err: errors.New("boot failed")} }
	exitFunc = func(code int) {
		exited = true
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	}
	t.Cleanup(func() {
		newAppRunner = oldRunner
		exitFunc = oldExit
	})
	main()
	if !exited {
		t.Fatal("expected main to exit on failure")
	}
}

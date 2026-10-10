package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestExampleOutput(t *testing.T) {
	var output bytes.Buffer
	if err := run(&output); err != nil {
		t.Fatal(err)
	}
	want := "fresh: freight=21 delivery=2d notify=standard\n" +
		"fresh: freight=0 delivery=1d notify=express\n" +
		"retail: freight=6 delivery=1d notify=retail\n"
	if output.String() != want {
		t.Fatalf("got %q, want %q", output.String(), want)
	}
}

type failedWriter struct{}

var errWrite = errors.New("write failed")

func (failedWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestExamplePropagatesWriterError(t *testing.T) {
	if err := run(failedWriter{}); !errors.Is(err, errWrite) {
		t.Fatalf("error = %v", err)
	}
	if err := run(io.Discard); err != nil {
		t.Fatal(err)
	}
}

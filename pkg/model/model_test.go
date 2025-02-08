package model

import (
	"os"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

func TestLoadConfig(t *testing.T) {
	// dir, _ := os.Getwd()
	// filepath := dir + "/test.yaml"
	filepath := "test.yaml"
	_, err := types.LoadConfig(filepath)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadDot(t *testing.T) {
	dir, _ := os.Getwd()
	filepath := dir + "/test.dot"
	_, err := DiagramFromDotFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
}

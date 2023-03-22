package model

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// dir, _ := os.Getwd()
	// filepath := dir + "/test.yaml"
	filepath := "test.yaml"
	_, err := LoadConfig(filepath)
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

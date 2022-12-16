package model

import (
	"fmt"
	"os"
	"testing"

	"gonum.org/v1/gonum/graph"
)

func TestLoadConfig(t *testing.T) {
	// dir, _ := os.Getwd()
	// filepath := dir + "/test.yaml"
	filepath := "test.yaml"
	cfg, err := LoadConfig(filepath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg)
}

func TestLoadDot(t *testing.T) {
	dir, _ := os.Getwd()
	filepath := dir + "/test.dot"
	nd, err := NetworkDiagramFromDotFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", nd)
	for _, e := range graph.EdgesOf(nd.Edges()) {
		fmt.Printf("%+v\n", e)
	}
}

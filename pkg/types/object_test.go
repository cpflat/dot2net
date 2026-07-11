package types

import (
	"strings"
	"testing"
)

// newTestConfig returns a Config with all internal lookup maps initialized,
// so that AddNodeClass/AddInterfaceClass/... and the *ByName helpers can be
// used directly in unit tests without going through LoadConfig (YAML).
func newTestConfig() *Config {
	return &Config{
		fileDefinitionMap:  map[string]*FileDefinition{},
		formatStyleMap:     map[string]*FormatStyle{},
		layerMap:           map[string]*Layer{},
		policyMap:          map[string]*IPPolicy{},
		parameterRuleMap:   map[string]*ParameterRule{},
		nodeClassMap:       map[string]*NodeClass{},
		interfaceClassMap:  map[string]*InterfaceClass{},
		connectionClassMap: map[string]*ConnectionClass{},
		groupClassMap:      map[string]*GroupClass{},
		segmentClassMap:    map[string]*SegmentClass{},
		neighborClassMap:   map[string]map[string][]*NeighborClass{},
	}
}

// TestNode_SetClasses_ConflictDetection pins the value/prefix/mgmt conflict
// detection and the "compatible classes merge" behavior of Node.SetClasses.
// This is the cross-object composition logic flagged by CR-070 as high-risk
// and previously only covered indirectly by the example golden tests.
func TestNode_SetClasses_ConflictDetection(t *testing.T) {
	tests := []struct {
		name string
		// mgmtClasses are InterfaceClass names to register before the node classes.
		mgmtClasses []string
		classes     []*NodeClass
		// labels applied to the node, in order (defaults to all class names).
		labels     []string
		wantErr    string // substring; "" means success
		wantPrefix string // asserted only when wantErr == ""
		wantMgmt   string // expected mgmtInterfaceClass name; "" = none
	}{
		{
			name: "single class, no conflict",
			classes: []*NodeClass{
				{Name: "router", Values: map[string]string{"as": "65000"}, Prefix: "rt"},
			},
			wantPrefix: "rt",
		},
		{
			name:       "no classes falls back to default prefix",
			classes:    []*NodeClass{},
			labels:     []string{},
			wantPrefix: DefaultNodePrefix,
		},
		{
			name: "same value in two classes is compatible",
			classes: []*NodeClass{
				{Name: "a", Values: map[string]string{"as": "65000"}},
				{Name: "b", Values: map[string]string{"as": "65000"}},
			},
			wantPrefix: DefaultNodePrefix,
		},
		{
			name: "disjoint values in two classes are compatible",
			classes: []*NodeClass{
				{Name: "a", Values: map[string]string{"as": "65000"}},
				{Name: "b", Values: map[string]string{"role": "spine"}},
			},
			wantPrefix: DefaultNodePrefix,
		},
		{
			name: "conflicting values are rejected",
			classes: []*NodeClass{
				{Name: "a", Values: map[string]string{"as": "65000"}},
				{Name: "b", Values: map[string]string{"as": "65001"}},
			},
			wantErr: "different values for 'as'",
		},
		{
			name: "same prefix in two classes is compatible",
			classes: []*NodeClass{
				{Name: "a", Prefix: "sw"},
				{Name: "b", Prefix: "sw"},
			},
			wantPrefix: "sw",
		},
		{
			name: "prefix set by one class, empty in the other",
			classes: []*NodeClass{
				{Name: "a", Prefix: "sw"},
				{Name: "b"},
			},
			wantPrefix: "sw",
		},
		{
			name: "conflicting prefixes are rejected",
			classes: []*NodeClass{
				{Name: "a", Prefix: "sw"},
				{Name: "b", Prefix: "rt"},
			},
			wantErr: "different prefix",
		},
		{
			name:        "valid mgmt interface class is resolved",
			mgmtClasses: []string{"mgmt"},
			classes: []*NodeClass{
				{Name: "a", MgmtInterface: "mgmt"},
			},
			wantPrefix: DefaultNodePrefix,
			wantMgmt:   "mgmt",
		},
		{
			name:        "conflicting mgmt interface classes are rejected",
			mgmtClasses: []string{"mgmtA", "mgmtB"},
			classes: []*NodeClass{
				{Name: "a", MgmtInterface: "mgmtA"},
				{Name: "b", MgmtInterface: "mgmtB"},
			},
			wantErr: "different management interface",
		},
		{
			name: "undefined mgmt interface class is rejected",
			classes: []*NodeClass{
				{Name: "a", MgmtInterface: "nope"},
			},
			wantErr: "invalid mgmt interface class",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			for _, mc := range tt.mgmtClasses {
				cfg.AddInterfaceClass(&InterfaceClass{Name: mc})
			}
			labels := tt.labels
			if labels == nil {
				for _, nc := range tt.classes {
					labels = append(labels, nc.Name)
				}
			}
			for _, nc := range tt.classes {
				cfg.AddNodeClass(nc)
			}

			nm := NewNetworkModel()
			node := nm.NewNode("n1")
			if err := node.SetLabels(cfg, labels, nil); err != nil {
				t.Fatalf("SetLabels: unexpected error: %v", err)
			}

			err := node.SetClasses(cfg, nm)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("SetClasses: expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("SetClasses: error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetClasses: unexpected error: %v", err)
			}
			if node.NamePrefix != tt.wantPrefix {
				t.Errorf("NamePrefix = %q, want %q", node.NamePrefix, tt.wantPrefix)
			}
			if tt.wantMgmt == "" {
				if node.mgmtInterfaceClass != nil {
					t.Errorf("mgmtInterfaceClass = %q, want none", node.mgmtInterfaceClass.Name)
				}
			} else {
				if node.mgmtInterfaceClass == nil || node.mgmtInterfaceClass.Name != tt.wantMgmt {
					t.Errorf("mgmtInterfaceClass = %v, want %q", node.mgmtInterfaceClass, tt.wantMgmt)
				}
			}
		})
	}
}

// TestNode_SetLabels_UndefinedClass verifies the strict/lenient handling of an
// undefined class label, controlled by GlobalSettings.IgnoreUndefinedClass
// (the CR-006 switch). In strict mode an undefined class is a hard error; in
// lenient mode it is skipped (kept as a class label but not resolved).
func TestNode_SetLabels_UndefinedClass(t *testing.T) {
	t.Run("strict rejects undefined class", func(t *testing.T) {
		cfg := newTestConfig()
		nm := NewNetworkModel()
		node := nm.NewNode("n1")
		err := node.SetLabels(cfg, []string{"ghost"}, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid nodeclass name") {
			t.Fatalf("expected 'invalid nodeclass name' error, got %v", err)
		}
	})

	t.Run("lenient skips undefined class", func(t *testing.T) {
		cfg := newTestConfig()
		cfg.GlobalSettings.IgnoreUndefinedClass = true
		nm := NewNetworkModel()
		node := nm.NewNode("n1")
		if err := node.SetLabels(cfg, []string{"ghost"}, nil); err != nil {
			t.Fatalf("unexpected error in lenient mode: %v", err)
		}
		if len(node.GetClasses()) != 0 {
			t.Errorf("expected no resolved classes, got %d", len(node.GetClasses()))
		}
		if !node.HasClass("ghost") {
			t.Errorf("expected 'ghost' to remain as a class label")
		}
	})
}

// TestGroup_SetGroupRelativeParams pins the key composition and priority
// ("first writer wins") of Group.SetGroupRelativeParams, called out by CR-070
// as the group-precedence logic (node-num > smaller-group-num > larger-group-num).
func TestGroup_SetGroupRelativeParams(t *testing.T) {
	t.Run("composes group_ prefix and per-class aliases", func(t *testing.T) {
		nm := NewNetworkModel()
		g := nm.NewGroup("g1")
		g.AddParam("id", "5")
		g.ParsedLabels = newParsedLabels()
		g.AddClassLabels("vlan", "l2")

		target := nm.NewNode("target")
		if err := g.SetGroupRelativeParams(target, ""); err != nil {
			t.Fatalf("SetGroupRelativeParams: %v", err)
		}

		rp := target.GetRelativeParams()
		for key, want := range map[string]string{
			"group_id": "5",
			"vlan_id":  "5",
			"l2_id":    "5",
		} {
			if got := rp[key]; got != want {
				t.Errorf("relative param %q = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("header prefix is prepended", func(t *testing.T) {
		nm := NewNetworkModel()
		g := nm.NewGroup("g1")
		g.AddParam("id", "5")
		g.ParsedLabels = newParsedLabels()
		g.AddClassLabels("vlan")

		target := nm.NewNode("target")
		if err := g.SetGroupRelativeParams(target, NumberPrefixOppositeInterface); err != nil {
			t.Fatalf("SetGroupRelativeParams: %v", err)
		}

		rp := target.GetRelativeParams()
		if got := rp["opp_group_id"]; got != "5" {
			t.Errorf("opp_group_id = %q, want %q", got, "5")
		}
		if got := rp["opp_vlan_id"]; got != "5" {
			t.Errorf("opp_vlan_id = %q, want %q", got, "5")
		}
	})

	t.Run("existing relative param is not overwritten (first writer wins)", func(t *testing.T) {
		nm := NewNetworkModel()
		g := nm.NewGroup("g1")
		g.AddParam("id", "5")
		g.ParsedLabels = newParsedLabels()
		g.AddClassLabels("vlan")

		target := nm.NewNode("target")
		// Simulate a higher-priority writer (e.g. node or smaller group) having
		// already populated group_id; the group must not clobber it.
		target.SetRelativeParam("group_id", "99")

		if err := g.SetGroupRelativeParams(target, ""); err != nil {
			t.Fatalf("SetGroupRelativeParams: %v", err)
		}

		rp := target.GetRelativeParams()
		if got := rp["group_id"]; got != "99" {
			t.Errorf("group_id = %q, want preserved %q", got, "99")
		}
		// The class alias was not pre-set, so it is still populated.
		if got := rp["vlan_id"]; got != "5" {
			t.Errorf("vlan_id = %q, want %q", got, "5")
		}
	})
}

// TestInterface_BuildRelativeNameSpace_PrefixComposition pins the relative
// namespace prefix synthesis (self / node_ / opp_ / opp_node_ / group_) that
// CR-070 identifies as the highest-risk, previously-unverified path.
func TestInterface_BuildRelativeNameSpace_PrefixComposition(t *testing.T) {
	nm := NewNetworkModel()

	nodeA := nm.NewNode("a")
	nodeA.ParsedLabels = newParsedLabels()
	nodeA.AddParam("nid", "1")
	ifA := nodeA.NewInterface("eth0")
	ifA.ParsedLabels = newParsedLabels()
	ifA.AddParam("iid", "10")

	// A group on nodeA exercises the interface -> node-group relative path.
	g := nm.NewGroup("g")
	g.AddParam("gid", "7")
	g.ParsedLabels = newParsedLabels()
	nodeA.Groups = append(nodeA.Groups, g)

	nodeB := nm.NewNode("b")
	nodeB.ParsedLabels = newParsedLabels()
	ifB := nodeB.NewInterface("eth0")
	ifB.ParsedLabels = newParsedLabels()
	ifB.AddParam("iid", "20")
	nodeB.AddParam("nid", "2")

	// Wire the point-to-point connection and the opposite back-references
	// (NewConnection sets Connection but not Opposite).
	nm.NewConnection(ifA, ifB)
	ifA.Opposite = ifB
	ifB.Opposite = ifA

	if err := ifA.BuildRelativeNameSpace(map[string]map[string]string{}); err != nil {
		t.Fatalf("BuildRelativeNameSpace: %v", err)
	}

	rp := ifA.GetRelativeParams()
	want := map[string]string{
		"iid":          "10", // self interface
		"node_nid":     "1",  // self node
		"group_gid":    "7",  // self node group
		"opp_iid":      "20", // opposite interface
		"opp_node_nid": "2",  // opposite node
	}
	for key, val := range want {
		if got := rp[key]; got != val {
			t.Errorf("relative param %q = %q, want %q", key, got, val)
		}
	}
}

// TestNode_BuildRelativeNameSpace pins the node-level self + group prefix
// synthesis independently of interfaces.
func TestNode_BuildRelativeNameSpace(t *testing.T) {
	nm := NewNetworkModel()
	node := nm.NewNode("n1")
	node.ParsedLabels = newParsedLabels()
	node.AddParam("nid", "3")

	g := nm.NewGroup("g1")
	g.AddParam("gid", "8")
	g.ParsedLabels = newParsedLabels()
	node.Groups = append(node.Groups, g)

	if err := node.BuildRelativeNameSpace(map[string]map[string]string{}); err != nil {
		t.Fatalf("BuildRelativeNameSpace: %v", err)
	}

	rp := node.GetRelativeParams()
	if got := rp["nid"]; got != "3" {
		t.Errorf("nid = %q, want %q", got, "3")
	}
	if got := rp["group_gid"]; got != "8" {
		t.Errorf("group_gid = %q, want %q", got, "8")
	}
}

// TestNetworkModel_ConstructionBoundaries covers the boundary topologies
// enumerated by CR-070: isolated node, self-loop, and multi-edge, plus the
// basic name-map wiring of NewNode/NewGroup/NewConnection.
func TestNetworkModel_ConstructionBoundaries(t *testing.T) {
	t.Run("NewNode wires name map", func(t *testing.T) {
		nm := NewNetworkModel()
		n := nm.NewNode("r1")
		if len(nm.Nodes) != 1 {
			t.Fatalf("Nodes len = %d, want 1", len(nm.Nodes))
		}
		got, ok := nm.NodeByName("r1")
		if !ok || got != n {
			t.Errorf("NodeByName(r1) = %v, %v; want the created node", got, ok)
		}
		if _, ok := nm.NodeByName("absent"); ok {
			t.Errorf("NodeByName(absent) should report missing")
		}
	})

	t.Run("NewGroup wires name map", func(t *testing.T) {
		nm := NewNetworkModel()
		g := nm.NewGroup("grp")
		got, ok := nm.GroupByName("grp")
		if !ok || got != g {
			t.Errorf("GroupByName(grp) = %v, %v; want the created group", got, ok)
		}
	})

	t.Run("isolated node has no interfaces and generates no files", func(t *testing.T) {
		cfg := newTestConfig()
		nm := NewNetworkModel()
		n := nm.NewNode("lonely")
		n.ParsedLabels = newParsedLabels()
		if len(n.Interfaces) != 0 {
			t.Errorf("isolated node has %d interfaces, want 0", len(n.Interfaces))
		}
		if files := n.FilesToGenerate(cfg); len(files) != 0 {
			t.Errorf("isolated classless node generates %v, want none", files)
		}
	})

	t.Run("self-loop: both endpoints on the same node", func(t *testing.T) {
		nm := NewNetworkModel()
		n := nm.NewNode("r1")
		i1 := n.NewInterface("eth0")
		i2 := n.NewInterface("eth1")
		conn := nm.NewConnection(i1, i2)
		if conn.Src.Node != conn.Dst.Node {
			t.Errorf("self-loop endpoints on different nodes: %v vs %v", conn.Src.Node, conn.Dst.Node)
		}
		if i1.Connection != conn || i2.Connection != conn {
			t.Errorf("both interfaces should reference the connection")
		}
	})

	t.Run("multi-edge: two connections between the same node pair", func(t *testing.T) {
		nm := NewNetworkModel()
		a := nm.NewNode("a")
		b := nm.NewNode("b")
		nm.NewConnection(a.NewInterface("eth0"), b.NewInterface("eth0"))
		nm.NewConnection(a.NewInterface("eth1"), b.NewInterface("eth1"))
		if len(nm.Connections) != 2 {
			t.Errorf("Connections len = %d, want 2", len(nm.Connections))
		}
		if len(a.Interfaces) != 2 || len(b.Interfaces) != 2 {
			t.Errorf("each node should have 2 interfaces; a=%d b=%d", len(a.Interfaces), len(b.Interfaces))
		}
	})
}

// TestNode_FilesToGenerate_Sorted verifies that FilesToGenerate returns a
// deterministic, sorted list regardless of ConfigTemplate declaration order
// (regression guard for the CR-033 map-iteration determinism fix).
func TestNode_FilesToGenerate_Sorted(t *testing.T) {
	cfg := newTestConfig()
	cfg.AddNodeClass(&NodeClass{
		Name: "router",
		ConfigTemplates: []*ConfigTemplate{
			{File: "zebra.conf"},
			{File: "bgpd.conf"},
			{File: "ospfd.conf"},
		},
	})

	nm := NewNetworkModel()
	node := nm.NewNode("n1")
	if err := node.SetLabels(cfg, []string{"router"}, nil); err != nil {
		t.Fatalf("SetLabels: %v", err)
	}

	got := node.FilesToGenerate(cfg)
	want := []string{"bgpd.conf", "ospfd.conf", "zebra.conf"}
	if len(got) != len(want) {
		t.Fatalf("FilesToGenerate = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FilesToGenerate = %v, want %v (sorted)", got, want)
		}
	}
}

package model

import (
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// resolveDeployFor builds the model from the given config and DOT and returns
// how the named node ends up being deployed.
func resolveDeployFor(t *testing.T, yaml, dot, nodeName string) (string, error) {
	t.Helper()
	cfgPath, dotPath := writeTempInput(t, yaml, dot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := buildSkeleton(cfg, d)
	if err != nil {
		t.Fatalf("buildSkeleton: %v", err)
	}
	if err := checkClasses(cfg, nm); err != nil {
		return "", err
	}
	for _, n := range nm.Nodes {
		if n.Name == nodeName {
			return cfg.ResolveDeploy(n)
		}
	}
	t.Fatalf("node %s was not built", nodeName)
	return "", nil
}

// TestDeploySilentClassMakesNoClaim is the property the whole two-stage design
// exists for: a class that does not mention deploy must not be read as claiming
// "container". Were the empty value normalised to DeployContainer when the
// config is loaded, every ordinary class would claim it, and this node - which
// carries one silent class and one asking for platform - would fail as a
// same-tier conflict instead of becoming a platform node.
func TestDeploySilentClassMakesNoClaim(t *testing.T) {
	got, err := resolveDeployFor(t, `
name: deploy_silent
nodeclass:
  - name: named
    values:
      role: "sw"
  - name: on_platform
    deploy: platform
`, `graph { sw1 [class="named;on_platform"]; r1; sw1 -- r1; }`, "sw1")
	if err != nil {
		t.Fatalf("a silent class must not conflict with an explicit one, got: %v", err)
	}
	if got != types.DeployPlatform {
		t.Errorf("deploy = %q, want %q", got, types.DeployPlatform)
	}
}

// TestDeployFallsBackToContainer covers the node no class speaks for.
func TestDeployFallsBackToContainer(t *testing.T) {
	got, err := resolveDeployFor(t, `
name: deploy_default
nodeclass:
  - name: router
`, `graph { r1 [class="router"]; r2; r1 -- r2; }`, "r1")
	if err != nil {
		t.Fatalf("checkClasses: %v", err)
	}
	if got != types.DeployContainer {
		t.Errorf("deploy = %q, want %q", got, types.DeployContainer)
	}
}

// TestDeployUserClassBeatsBaseClass is what makes class_policy usable as a
// topology-wide default: the base class states the default and a class the user
// named on the node overrides it, rather than clashing with it.
func TestDeployUserClassBeatsBaseClass(t *testing.T) {
	yaml := `
name: deploy_tier
class_policy:
  node:
    base: [all]
nodeclass:
  - name: all
    deploy: platform
  - name: router
    deploy: container
`
	dot := `graph { r1 [class="router"]; sw1; r1 -- sw1; }`

	got, err := resolveDeployFor(t, yaml, dot, "r1")
	if err != nil {
		t.Fatalf("the base class should lose to the named class, got: %v", err)
	}
	if got != types.DeployContainer {
		t.Errorf("node r1: deploy = %q, want %q (the class the user named wins)", got, types.DeployContainer)
	}

	// The node that names no class still gets the topology-wide default.
	got, err = resolveDeployFor(t, yaml, dot, "sw1")
	if err != nil {
		t.Fatalf("checkClasses: %v", err)
	}
	if got != types.DeployPlatform {
		t.Errorf("node sw1: deploy = %q, want %q (from the base class)", got, types.DeployPlatform)
	}
}

// TestDeployWeighsTiersNotLabelOrder covers the case that makes resolution
// compare tiers instead of trusting the order of ClassLabels: a class pulled in
// through use: is appended after the base class, so walking the slice and
// keeping the first claim would let the weaker base class win - or, worse,
// report a conflict between two claims that are not of equal weight.
func TestDeployWeighsTiersNotLabelOrder(t *testing.T) {
	got, err := resolveDeployFor(t, `
name: deploy_use_order
class_policy:
  node:
    base: [all]
nodeclass:
  - name: all
    deploy: platform
  - name: router
    use: [as_container]
  - name: as_container
    deploy: container
`, `graph { r1 [class="router"]; r2; r1 -- r2; }`, "r1")
	if err != nil {
		t.Fatalf("a used user class outranks the base class, got: %v", err)
	}
	if got != types.DeployContainer {
		t.Errorf("deploy = %q, want %q (use: keeps the user tier of as_container)", got, types.DeployContainer)
	}
}

// TestDeploySameTierConflictIsAnError pins the other half of the tier rule:
// once two classes carry equal weight, only the user can say which is meant.
func TestDeploySameTierConflictIsAnError(t *testing.T) {
	_, err := resolveDeployFor(t, `
name: deploy_conflict
nodeclass:
  - name: as_container
    deploy: container
  - name: as_platform
    deploy: platform
`, `graph { sw1 [class="as_container;as_platform"]; r1; sw1 -- r1; }`, "sw1")
	if err == nil {
		t.Fatal("two user classes asking for different deploy forms must be an error")
	}
	if !strings.Contains(err.Error(), "deploy") && !strings.Contains(err.Error(), "container") {
		t.Errorf("the error should name the conflicting forms, got: %v", err)
	}
}

// TestDeployUnknownValueIsRejected keeps a typo from silently behaving like a
// container.
func TestDeployUnknownValueIsRejected(t *testing.T) {
	_, err := resolveDeployFor(t, `
name: deploy_typo
nodeclass:
  - name: broken
    deploy: platfrom
`, `graph { sw1 [class="broken"]; r1; sw1 -- r1; }`, "sw1")
	if err == nil {
		t.Fatal("an unknown deploy value must be rejected")
	}
	if !strings.Contains(err.Error(), "platfrom") {
		t.Errorf("the error should quote the offending value, got: %v", err)
	}
}

// TestVirtualNoLongerDecidesDeployment covers the change of meaning in v0.8.
// virtual withholds an object's configuration and says nothing about whether it
// is deployed, so a class written for v0.7 - where one flag answered both - is
// refused rather than read the old way or the new way silently.
func TestVirtualNoLongerDecidesDeployment(t *testing.T) {
	_, err := resolveDeployFor(t, `
name: deploy_virtual_alone
nodeclass:
  - name: vrouter
    virtual: true
`, `graph { v1 [class="vrouter"]; r1; v1 -- r1; }`, "v1")
	if err == nil {
		t.Fatal("virtual: true without deploy must be refused: it meant deploy: none in v0.7")
	}
	if !strings.Contains(err.Error(), types.DeployNone) {
		t.Errorf("the error must name the form to write instead, got: %v", err)
	}

	// The pair the old spelling could not express: a node that is deployed like
	// any other, whose configuration dot2net does not write.
	got, err := resolveDeployFor(t, `
name: deploy_virtual_with_form
nodeclass:
  - name: unwritten
    virtual: true
    deploy: container
`, `graph { r1 [class="unwritten"]; r2; r1 -- r2; }`, "r1")
	if err != nil {
		t.Fatalf("virtual alongside a deployment form must be allowed, got: %v", err)
	}
	if got != types.DeployContainer {
		t.Errorf("deploy = %q, want %q", got, types.DeployContainer)
	}
}

// TestVirtualFalseMakesNoClaim preserves how the boolean has always behaved: it
// cannot be used to contradict another class, because false is indistinguishable
// from unset. Writing deploy: container is the way to say it out loud.
func TestVirtualFalseMakesNoClaim(t *testing.T) {
	got, err := resolveDeployFor(t, `
name: deploy_virtual_false
nodeclass:
  - name: not_virtual
    virtual: false
  - name: on_platform
    deploy: platform
`, `graph { sw1 [class="not_virtual;on_platform"]; r1; sw1 -- r1; }`, "sw1")
	if err != nil {
		t.Fatalf("virtual: false must not conflict with anything, got: %v", err)
	}
	if got != types.DeployPlatform {
		t.Errorf("deploy = %q, want %q", got, types.DeployPlatform)
	}
}

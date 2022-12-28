package clab

// Containerlab topology config definition based on github.com/srl-labs/containerlab v0.34.0
type Config struct {
	Name     string    `json:"name,omitempty"`
	Prefix   *string   `json:"prefix,omitempty"`
	Mgmt     *MgmtNet  `json:"mgmt,omitempty"`
	Topology *Topology `json:"topology,omitempty"`
}

type MgmtNet struct {
	Network string `yaml:"network,omitempty" json:"network,omitempty"` // container runtime network name
	Bridge  string `yaml:"bridge,omitempty" json:"bridge,omitempty"`
	// linux bridge backing the runtime network
	IPv4Subnet     string `yaml:"ipv4_subnet,omitempty" json:"ipv4-subnet,omitempty"`
	IPv4Gw         string `yaml:"ipv4-gw,omitempty" json:"ipv4-gw,omitempty"`
	IPv6Subnet     string `yaml:"ipv6_subnet,omitempty" json:"ipv6-subnet,omitempty"`
	IPv6Gw         string `yaml:"ipv6-gw,omitempty" json:"ipv6-gw,omitempty"`
	MTU            string `yaml:"mtu,omitempty" json:"mtu,omitempty"`
	ExternalAccess *bool  `yaml:"external-access,omitempty" json:"external-access,omitempty"`
}

type Topology struct {
	Defaults *NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*NodeDefinition `yaml:"nodes,omitempty"`
	Links    []*LinkConfig              `yaml:"links,omitempty"`
}

type NodeDefinition struct {
	Kind                 string            `yaml:"kind,omitempty"`
	Group                string            `yaml:"group,omitempty"`
	Type                 string            `yaml:"type,omitempty"`
	StartupConfig        string            `yaml:"startup-config,omitempty"`
	StartupDelay         uint              `yaml:"startup-delay,omitempty"`
	EnforceStartupConfig bool              `yaml:"enforce-startup-config,omitempty"`
	AutoRemove           *bool             `yaml:"auto-remove,omitempty"`
	Config               *ConfigDispatcher `yaml:"config,omitempty"`
	Image                string            `yaml:"image,omitempty"`
	License              string            `yaml:"license,omitempty"`
	Position             string            `yaml:"position,omitempty"`
	Entrypoint           string            `yaml:"entrypoint,omitempty"`
	Cmd                  string            `yaml:"cmd,omitempty"`
	// list of subject Alternative Names (SAN) to be added to the node's certificate
	SANs []string `yaml:"SANs,omitempty"`
	// list of commands to run in container
	Exec []string `yaml:"exec,omitempty"`
	// list of bind mount compatible strings
	Binds []string `yaml:"binds,omitempty"`
	// list of port bindings
	Ports []string `yaml:"ports,omitempty"`
	// user-defined IPv4 address in the management network
	MgmtIPv4 string `yaml:"mgmt_ipv4,omitempty"`
	// user-defined IPv6 address in the management network
	MgmtIPv6 string `yaml:"mgmt_ipv6,omitempty"`
	// list of ports to publish with mysocketctl
	Publish []string `yaml:"publish,omitempty"`
	// environment variables
	Env map[string]string `yaml:"env,omitempty"`
	// external file containing environment variables
	EnvFiles []string `yaml:"env-files,omitempty"`
	// linux user used in a container
	User string `yaml:"user,omitempty"`
	// container labels
	Labels map[string]string `yaml:"labels,omitempty"`
	// container networking mode. if set to `host` the host networking will be used for this node, else bridged network
	NetworkMode string `yaml:"network-mode,omitempty"`
	// Ignite sandbox and kernel imageNames
	Sandbox string `yaml:"sandbox,omitempty"`
	Kernel  string `yaml:"kernel,omitempty"`
	// Override container runtime
	Runtime string `yaml:"runtime,omitempty"`
	// Set node CPU (cgroup or hypervisor)
	CPU float64 `yaml:"cpu,omitempty"`
	// Set node CPUs to use
	CPUSet string `yaml:"cpu-set,omitempty"`
	// Set node Memory (cgroup or hypervisor)
	Memory string `yaml:"memory,omitempty"`
	// Set the nodes Sysctl
	Sysctls map[string]string `yaml:"sysctls,omitempty"`
	// Extra options, may be kind specific
	Extras *Extras `yaml:"extras,omitempty"`
	// List of node names to wait for before satarting this particular node
	WaitFor []string `yaml:"wait-for,omitempty"`
}

type LinkConfig struct {
	Endpoints []string               `yaml:"endpoints,flow"`
	Labels    map[string]string      `yaml:"labels,omitempty"`
	Vars      map[string]interface{} `yaml:"vars,omitempty"`
}

type ConfigDispatcher struct {
	Vars map[string]interface{} `yaml:"vars,omitempty"`
}

type Extras struct {
	SRLAgents []string `yaml:"srl-agents,omitempty"`
	// Nokia SR Linux agents. As of now just the agents spec files can be provided here
	MysocketProxy string `yaml:"mysocket-proxy,omitempty"`
	// Proxy address that mysocketctl will use
	CeosCopyToFlash []string `yaml:"ceos-copy-to-flash,omitempty"`
	// paths to files which are to be copied to ceos flash dir
}

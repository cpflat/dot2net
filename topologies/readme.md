# Topologies

Each directory here is one **dot2net topology**: an `input.dot` and an
`input.yaml` that together describe a network you can generate and deploy.
Build one by running dot2net inside its directory:

```bash
cd topologies/ospf_simple
../../dot2net build -c input.yaml input.dot
```

These are the ones worth actually running. The notation demonstrations — small
inputs written to show what a piece of syntax does — live in [`example/`](../example)
instead.

Every directory carries an `expected/` holding the output dot2net should
produce, which `internal/test/example_test.go` checks on every build.


## Corresponding to [FRR](https://frrouting.org/) [topotests](https://github.com/FRRouting/frr/tree/master/tests/topotests/)

- ospf_topo1
- ospf6_topo1
- rip_topo1
- bgp_features
- bgp_evpn_vxlan_topo1


## Corresponding to [TiNET](https://github.com/tinynetwork/tinet) [examples](https://github.com/tinynetwork/tinet/tree/master/examples)

- basic_bgp
- basic_clos
- basic_ospfv2_frr
- basic_ospfv3_frr


## Written for dot2net

- ospf_simple — the smallest thing that routes. Start here.
- ospf_multihost — ospf_simple placed on two machines. The one to copy when
  writing a multi-host topology.
- aggregate_crossing — one shared segment reaching two machines, drawn once.
  Shows what a link leaving a machine costs and how the count is brought down.
- kathara_basic — a shared medium and a point-to-point link on Kathara.


## Platforms

Every topology here generates for containerlab and TiNET. Most generate for
Kathara as well; the ones that do not say why where their modules are listed:

- `basic_clos`, `bgp_evpn_vxlan_topo1` name their own interfaces, and Kathara
  derives an interface's name from its index in lab.conf
- `ospf_multihost`, `aggregate_crossing` place nodes on more than one machine,
  which a lab.conf cannot express


## Elsewhere

`basic_mpls`, `large_clos` and `large_ring` left when v0.4.0 changed the
configuration format under them. They are written for v0.3.6 and do not run on
this version, and are kept outside this repository.

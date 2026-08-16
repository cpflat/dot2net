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

Every topology here generates for containerlab and TiNET, and all but one for
Kathara too.

**`basic_clos` is the exception**, and the reason is worth knowing before you
write your own: it names its own interfaces (`up1`, `dn1`), and those names
belong to interfaces that are ends of real links. Kathara derives an interface's
name from its index in `lab.conf`, so a link's end is `eth0`, `eth1`, ... and
nothing else — a name of the topology's choosing cannot be honoured. dot2net says
so while generating rather than letting the lab fail to start.

The rule reaches only the interfaces Kathara lists. A device the node's own
configuration builds — `deploy: logical`, such as the VXLAN bridges in
`bgp_evpn_vxlan_topo1` — never gets a line in `lab.conf`, so Kathara has no say
in its name and the topology keeps the one it chose. That is why naming an
interface is not by itself a reason a topology cannot run on Kathara.

See [Module: Kathara](https://github.com/cpflat/dot2net/wiki/Module-Kathara) for
what else it cannot express.


## Elsewhere

`basic_mpls`, `large_clos` and `large_ring` left when v0.4.0 changed the
configuration format under them. They are written for v0.3.6 and do not run on
this version, and are kept outside this repository.

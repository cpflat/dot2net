# Examples

Each directory here is a small **dot2net topology** written to show what one
piece of the notation does. They are meant to be read next to the
[Wiki](https://github.com/cpflat/dot2net/wiki), and they build the same way
anything else does:

```bash
cd example/switching
../../dot2net build -c input.yaml input.dot
```

They are deliberately minimal, and most configure no routing — deploying one
would not show you much. The topologies worth actually running live in
[`topologies/`](../topologies).

Every directory carries an `expected/` holding the output dot2net should
produce, which `internal/test/example_test.go` checks on every build. That is
why an example is worth adding even when it only demonstrates a notation: it
also pins the behaviour.


## Platforms

Every example here generates for containerlab and TiNET, except the two that
name no platform at all: `value_class_basic` and `test_blocks_basic` are about
the notation and stop at the config blocks. `deploy_and_virtual` generates for
all three, on purpose: what it demonstrates is decided per platform, so seeing
one of them would not tell you it was right.

One does not generate for Kathara, and says why where its modules are listed:
`address_reservation` names its own interfaces (up1, dn1), and Kathara derives
an interface's name from its index in lab.conf.

## What each one shows

- **deploy_and_virtual** — every form `deploy` can take, and what `virtual`
  withholds: a switch the platform provides, a node nobody deploys, a tunnel
  with no wire, a node that is deployed and left unconfigured, and an interface
  that is wired and silent
- **switching** — a switch node, and the segments that form around it
- **address_reservation** — reserving an address so the automatic assignment
  works around it
- **param_share** — one generated parameter read from more than one object,
  through the `conn_` and `node_` cross-object prefixes
- **value_class_basic** — the Value class, which attaches several parameter sets
  to one object and is how a template avoids a loop
- **test_blocks_basic** — `blocks.before` / `blocks.after`, which place a config
  block relative to the others

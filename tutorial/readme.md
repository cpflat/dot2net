# dot2net tutorial for container-based network

# Setup

This tutorial will introduce the way to effectively deploy container-based network with dot2net.
It requires docker environment and sudo privilege on your Linux computer.
In addition, it accepts Containerlab and TiNET as the platform to deploy container-based networks.

[Containerlab](https://containerlab.dev/)

[TiNET](https://github.com/tinynetwork/tinet)

## Input files for dot2net

There are two main files `input.dot` and `input.yaml` in this directory.
These files are the input for dot2net (file names can be changed).
DOT is a graph description language used in [Graphviz](https://graphviz.org/).
If you have installed graphviz, you can generate a visualized graph from `input.dot` with the following command.

    dot -Tpdf input.dot > input.pdf

Comparing the `input.dot` and the generated visualized graph,
you can see lines 2-4 in the `input.dot` defines the nodes and lines 6-7 defines the links between the nodes.
The detailed DOT grammar is explained in the [Graphviz documentation](https://graphviz.org/doc/info/lang.html).

You will also see that there is an attribute `class=router` for each node.
It means the node belongs to an ObjectClass `router`.
The ObjectClass is defined in `input.yaml`.

`input.yaml` defines what kind of configuration files to generate based on the given DOT file.
It includes files definitions to generate, IP address assignment policy, and ObjectClass definitions (with container image and config template blocks).
The files are generated for each nodes respectively, and binded to the container at the given path.
In this case, `daemons` and `vtysh.conf` will be placed as is in this directory, and `frr.conf` will be generated with the config templates.
The embedded parameters are automatically assigned based on the policied given in the `input.yaml`.

There are two kinds of ObjectClass definitions: `nodeclass` and `interfaceclass`.
They are corresponding to the nodes and links in the DOT file.
If the nodes and links have no attributes specifying the classes (e.g., links in `input.dot`),
the objects will belong to `default` class if defined
(i.e., the links in `input.dot` belong to `default` interface class).


## How to use dot2net and containerlab?

Let's generate configuration files from these files.

    dot2net build -c ./input.yaml ./input.dot

This command generates one directory per node, and one set of files per
platform:

- `topo.yaml`, a containerlab topology file
- `spec.yaml`, a TiNET specification file
- `kathara/`, a Kathara lab directory

All three come from the same `input.dot` and `input.yaml`; nothing in them names
a platform twice. Each node's directory holds the files `input.yaml` defines for
it, laid out by the path they take inside the container - `r1/etc/frr/frr.conf`,
for instance.

There are also `containerlab.sh`, `tinet.sh` and `kathara/kathara.sh`. Each one
runs its own platform's commands for you, and is described below.

(If your dot2net executable is not registered in your PATH, you need to replace `dot2net` to the executable path. e.g., `/path/to/dot2net`)

Here, let us use Containerlab primarily for explanation.
The topology file specifies nodes (with container image and file bind mount configurations) and link placement.
The topology file definition is explained at the [Containerlab documentation](https://containerlab.dev/manual/topo-def-file/).

Then try deploying the container network with following command.

    sudo ./containerlab.sh deploy

You can check the containers successfully deployed with `docker ps` command.
You can also reach a node through the script, which finds the container for you:

    sudo ./containerlab.sh exec r1 vtysh -c "show ip ospf neighbor"

Please check that the node `r1` is accessible with the node `r3` using ping command.

After testing the network, you can remove the network with following command.

    sudo ./containerlab.sh destroy

The script is not required - `sudo containerlab deploy -t topo.yaml` and
`sudo containerlab destroy -t topo.yaml --cleanup` do the same deploying and
undeploying. What the script adds is everything around them: it runs whatever
the topology asks to be run inside a node before the node goes, copies out the
files the topology asks to keep, and removes what the lab left on the machine.


## Change network topology with Containerlab

Traditionally, you can change the network topology by modifying the Containerlab topology file.
Try extending one node `r4` and one link between `r3` and `r4`.
You need to modify topo.yaml and generate directory `r4` including the internal files.
You also need to modify parameters in `r4/etc/frr/frr.conf` considering the IP address assignment.
In addition, do not forget to add new neighbor network configuration on `r3/etc/frr/frr.conf`

After that, try deploying the extended topology with containerlab and test it with ping.
If it fails, good luck finding out the reason and fix the configuration files.


## Change network topology with dot2net

You should change the network topology easier with dot2net.
You need to modify the `input.dot` to add node `r4` (with label `router`) and one link `r3->r4` (or `r4->r3` is also ok).
Note that you do not need to modify `input.yaml` in this case.

With the change, try generating `topo.yaml` again.
The generated configuration file will have automatically calculated parameters.

Try deploying the extended topology again.
There should be no failures, but try troubleshooting it if fails.


## Try a larger, real topology

The point of the previous section is not that adding one node is easy. It is
that nothing in `input.yaml` had to know how many nodes there are, or which one
is next to which. So a much bigger graph should work the same way.

`Sinet.dot` in this directory is one: [SINET](https://www.sinet.ad.jp/), the
Japanese research network, as recorded by the
[Topology Zoo](http://www.topology-zoo.org/) - 74 routers and 76 links. Every
node in it carries the same `xlabel="router"` your three routers carry, and
nothing else. Generate it with the very same `input.yaml`:

    dot2net build -c input.yaml Sinet.dot

You get 74 node directories, a `topo.yaml` and a `spec.yaml`, with every
address, interface name and OSPF network statement worked out. Deploy it the
same way as before if you have the memory for it - it is 74 containers, so give
the machine a few GB - and the routers will find each other across the whole
graph.

That is the whole idea: the graph is the input, and the configuration follows
from it.

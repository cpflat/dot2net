# dot2net

Dot2net generates config files for large-scale emulation networks
from a network topology (in DOT language) and configuration templates (in YAML).
It automatically calculate and assign required parameters such as IP addresses to be embedded in the config,
so you only need to modify the topology graph when you want to chenge the network layout.
Dot2net currently supports [TiNET](https://github.com/tinynetwork/tinet)
and [Containerlab](https://containerlab.dev/) as an emulation network platform. 


dot2net Tutorial: [English](https://github.com/cpflat/dot2net-evaluation/tree/master/tutorial), [Japanese](https://github.com/cpflat/dot2net-evaluation/blob/master/tutorial/readme-ja.md)



# Overview

![flow](image/flow.png)



# Usage


## Build on local go environment

    go build .

    // mv dot2net /usr/local/bin/dot2net  (if required)

## Build on Docker

    docker run --rm -i -t -v $PWD:/v -w /v golang:1.18 go build

## Deploy a network with TiNET

    // Generate tinet specification file
    dot2net tinet -c ./example/rip_topo1/rip.yaml ./example/rip_topo1/rip.dot > spec.yaml
    
    // Deploy
    tinet up -c spec.yaml | sudo sh -x
    tinet conf -c spec.yaml | sudo sh -x

    // Destroy
    tinet down -c spec.yaml | sudo sh -x

## Deploy a network with Containerlab

    // Generate containerlab topology file
    dot2net clab -c ./example/rip_topo1/rip.yaml ./example/rip_topo1/rip.dot > topo.yaml
    
    // Deploy
    sudo containerlab deploy --topo topo.yaml

    // Destroy
    sudo containerlab destroy --topo topo.yaml
 
 
## Show IPaddress assignment visualization

    // Generate DOT file of address assignment, and generate PDF of the DOT
    dot2tinet visual -c ./example/rip_topo1/rip.yaml ./example/rip_topo1/rip.dot | dot -Tpdf > addr.pdf


# DOT files

    digraph  {
        n1;
        n2;
        n1->n2;
    }

The DOT files are considered as a "directed multigraph", which means that it allows multiple lines between a pair of nodes.
A node corresponds to a device node (a container or a bridge),
and a line corresponds to a connection between two nodes.

If all links have same meanings (and configurations), the DOT file only needs node name labels.

    digraph  {
        n1;
        n2;
        n1:eth0->n2:eth0;
    }

Interface names can be specified in port fields if needed
(It may cause warnings in other usages such as dot commands).

    digraph  {
        subgraph cluster1 {
            n1;
            n2;
            n1:eth0->n2:eth0;
        }
    }

Subgraph clusters can be defined as node groups.
It can be used for automated AS number assignment.


## Class labels

    digraph {
        n1;
        n2[xlabel="a"];
        n3[class="a"];
        n1->n2[label="b"];
        n2->n3[headlabel="b;c"];
        
        subgraph cluster1 {
            label="s"
            n4;
            n5;
        }
    }

Nodes or links of different configuration can be specified with classes.
The classes can be specified like tags; There can be multiple tags for one node or link (separated with ";" or ",").

There are 3 kinds of classes.
- Node Class: Specified in "xlabel", "class", "conf", or "info" of nodes (Note that "label" is not included).
- Interface Class: "headlabel", "headclass", "headconf", or "headinfo" specifies arrow-head-side interface class of the link. "taillabel", "tailclass", "tailconf", or "tailinfo" specified arrow-tail-side interface class of the link.
- Connection Class: Specified in "label", "class", "conf", or "info" of links. It just means two ends of interfaces have same configuration.
- Group Class: Specified in "label", "class", "conf", or "info" of subgraphs.

For example in the above DOT, the interface of n2 connected with n3 belongs to two Interface Classes, b and c.
The definition of these classes are defined in the config file.

If no labels are given, they refer "default" classes if exists.
Also, if "all" classes are defined, they affects all possible objects (nodes or interfaces).

In addition, one object only have one primary class label.
Primary class can define node setup information (e.g., node docker image).
There can be multiple non-primary class labels,
but the non-Primary classes cannot include these setup information.


# Class definitions

The class definitions (YAML file) includes config template blocks and some flags.

The config templates can be specified inline (anyclass.config.template) or in external files (anyclass.config.file).


## Variables

Dot2net basically assigns automatically generated parameters.
There are two kind of generated parameters: Integer-based parameters and IP-related parameters.

Integer-based parameter rules are defined in "param_rule".
If the name of param_rule is assigned to "anyclass.params",
the corresponding parameters are generated for the class instances (and registered in their namespaces).

IP-related parameter rules are defined as "layer" rules.
There can be one IP address for each network objects in a layer.
In each layer definitions, there can be multiple "policy" items, which means the IP addressing policy (address ranges and prefix length).
IP addresses are assigned to the objects with policy names given in "anyclass.policy" or "anyclass.params".
Also, if the policy names are given in "nodeclass.interface_policy",
the all interfaces of the nodeclass instances will obtain the IP addresses of the policy.


## Variable replacers

Config templates of dot2net basically follow [text/template](https://pkg.go.dev/text/template) notation.

In dot2net, the available parameters are stored in relative namespaces.
Basically each object (with config template blocks) has a namespace
that includes the parameters of the object itself and other related objects.

Following parameters are always available regardless of the parameter flags.

| Class     | Number | Replacer | Note
|:----------|:-------|:----------------|--------
| Node      | (none) | name     | Node name
| Interface | (none) | name     | Interface name

For example, {{ .name }} in node config templates embeds the node name,
and {{ .name }} in interface (or connection) config templates embeds the corresponding interface name.

Integer-based parameters can also be available as the replacers of rule names.
If we define "as" rule in a node,
{{ .as }} will be replaced with the assigned "as" integer value.

In the case of IP-related parameters, there are multiple parameters for IP-address-aware objects.
These parameters are specified with the corresponding layer names.

| Class     | Replacer           | Note
|:----------|:-------------------|--------
| Node      | [Layer]_loopback | Loopback IP addresses, assigned for IPSpaces with loopback_range
| Interface | [Layer]_addr     | IP address (e.g., 192.0.2.1)
| Interface | [Layer]_net      | IP network (e.g., 192.0.2.0/24)
| Interface | [Layer]_plen     | IP prefix length (e.g., 24)

For example, if their is one defined layer named "ip",
{{ .ip_addr }} embeds the IP address automatically calculated on the IPSpace.

For config templates of Interface Classes or Connection Classes,
Relative prefix can be additionally used for specifying neighbor parameters.

| Class     | Prefix          | Note
|:----------|:----------------|:-------
| Node      | (none)          | Value of node itself
| Node      | group_          | Group value of node
| Node      | (groupcls)_     | Group value corresponding to specified group class of node
| Interface | (none)          | Value of interface itself
| Interface | node_           | Node value of interface
| Interface | group_          | Node group value of inteface
| Interface | opp_            | Value of opposite interface
| Interface | opp_node_       | Node value of opposite interface
| Interface | opp_group_      | Node group value of opposite interface
| Interface | opp_(groupcls)_ | Group value corresponding to specified group class of node

For example, {{ .opp_ip_net }} embeds IP network (such as "192.168.101.0/24")
of the opposite interface. 

The assigned parameters can be listed with "params" subcommand:

    dot2net params -c ./example/rip_topo1/rip.yaml ./example/rip_topo1/rip.dot

The relative namespace items can be listed with -a option:

    dot2net params -a -c ./example/rip_topo1/rip.yaml ./example/rip_topo1/rip.dot

The assigned parameters can also be output as JSON data with "data" subcommand:

    dot2net data -c ./example/rip_topo1/rip.yaml ./example/rip_topo1/rip.dot


# Advanced settings

## Multiple DOT input

You can specify multiple DOT files in dot2net command arguments.
This feature is simply for easier management of dot2net topology files
(any network topology can be described in one DOT file in theory).

If multiple DOT files are given,
dot2net will use all the nodes and links in the files.
When there are nodes of same name, the nodes are considered as an identical node.
When there are links beteeen interfaces of same name on same nodes,
the links are considered as an identical link.
For the identical nodes and links, dot2net simply merge the assigned labels.

Note that the identical nodes or links in multiple DOT file must be named manually on DOT files.

## Extended labels on DOT files

    digraph {
        n1[label="value=120"];
        n2[label="@n2"];
        n3[label="@n3"];
        n4[label="@target=n3"];
        n5[label="@target=n2"];
    }

Class labels have a limited namespace (see subsection "Variable replacers"),
which covers the node itself (for Node Class) or the opposite interfaces against the connection (for Interface and Connection Class).
For complecated or asymmetrical network configurations, there exist three kinds of extended labels.
These labels can also be specified in the DOT fields same as the Class labels,
and they can be mixed with Class labels or other labels if appropriately separated (with ";" or ",").

Value labels directly specifiy variables that can be used in the config templates.
The format separates variable name and the corresponding value with "=".
For example, {{ .value }} will be replacerd with 120 on the config template of node n1.
The value specified with Value label is only available in namespase of the specified object
(for example, "value" is available on templates of node n1, but NOT available on templates of the interfaces of node n1).

Place labels make the object referrable from any other objects.
The format has a prefix "@".
For example, any nodes or interfaces can embed IPAddress of n2 with {{ .n2_ip_addr }}.
Place labels cannot exist on Connections (only available on Nodes or Interfaces).

Meta Value labels define aliases to Place labels. 
The format also has a prefix "@", and the alias name and existing Place label name is separated with "=".
For example, {{ .target_name }} is replaced with "n3" on n4, and "n2" on n5.

## Configration ordering

If the configuration have expected order coming from its dependency,
you can set priority values for config templates.
If priority value is smaller, the config blocks will be on the head of merged configuration.
The default value of priority is 0,
which means you can set positive values to place configs on the tail and negative values to place them on the head.


# Reference

This tool has been demonstrated at [IEEE Transactions on Network and Service Management](https://doi.org/10.1109/TNSM.2025.3582212) (Early Access) and [CNSM2023](https://doi.org/10.23919/CNSM59352.2023.10327865).

If you use this code, consider citing:

    @article{Kobayashi_dot2net2025,
        author={Kobayashi, Satoru and Shiiba, Ryusei and Miwa, Shinsuke and Miyachi, Toshiyuki and Fukuda, Kensuke},
        journal={IEEE Transactions on Network and Service Management}, 
        title={Topology-Driven Configuration of Emulation Networks With Deterministic Templating}, 
        volume={},
        number={},
        pages={1-14},
        year={2025}
    }
    
    @inproceedings{Kobayashi_dot2net2023,
        author={Kobayashi, Satoru and Shiiba, Ryusei and Miura, Ryosuke and Miwa, Shinsuke and Miyachi, Toshiyuki and Fukuda, Kensuke},
        booktitle={19th International Conference on Network and Service Management (CNSM)}, 
        title={dot2net: A Labeled Graph Approach for Template-Based Configuration of Emulation Networks}, 
        pages={1-9},
        year={2023}
    }


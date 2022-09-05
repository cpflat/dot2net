# dot2tinet

Dot2tinet is one of [TiNET](https://github.com/tinynetwork/tinet) support tool.
It generates a tinet specification file from a network topology (specified in DOT language)
and config templates.
It automatically generates some numbers, such as IP addresses, AS numbers, etc, to be embedded in the config.


# Usage

## Build

    docker run --rm -i -t -v $PWD:/v -w /v golang:1.18 go build


## Generate specification file

    dot2tinet build -c ./example/basic_bgp/bgp.yaml ./example/basic_bgp/bgp.dot


# DOT files

    digraph  {
        n1;
        n2;
        n1->n2;
    }

A node corresponds to a device node (a container or a bridge),
and an edge corresponds to a connection between two nodes.
If all links have same meanings (and configurations), the DOT file only needs node name labels.

    digraph  {
        n1;
        n2;
        n1:eth0->n2:eth0;
    }

Interface names can be specified in port fields if needed
(It may cause warnings in other usages such as dot commands).

    digraph  {
        n1;
        n2[label="a"];
        n3[class="a"];
        n1->n2[label="b"];
        n2->n3[headlabel="b;c"];
    }

Nodes or edges of different configuration can be specified with classes.
The classes can be specified like tags; There can be multiple tags for one node or edge (separated with ";" or ",").
There are 3 kinds of classes.
- Node Class: Specified in "label" or "class" of nodes.
- Interface Class: Specified in "headlabel" or "taillabel" of edges.
- Connection Class: Specified in "label" of edges. It just means two ends of interfaces have same configuration.
For example in the above DOT, the interface of n2 connected with n3 belongs to two Interface Classes, b and c.
The definition of these classes are defined in the config file.

If no labels are given, they refer "default" classes if exists.
Also, if "all" classes are defined, they affects all possible objects (nodes or interfaces).

# Config templates

Config templates are defined in the definition of Classes.
They are specified inline (anyclass.config.template) or in external files (anyclass.config.file).

## Variable replacers

Config templates of dot2tinet basically follow [text/template](https://pkg.go.dev/text/template>) notation.
The number replacers can be available only when the corresponding number classifiers are specified in "anyclass.numbered" of the class.
The available numbers in the templates are following.

| Class     | Number | Replacer  | Note
|:----------|:-------|:---------|--------
| Node      | (none) | name     | Node name
| ^         | ip     | loopback | IP address from global.iploopback
| ^         | as     | as       | Private AS number
| Interface | (none) | name     | Interface name
| ^         | ip     | ipaddr   | IP address
| ^         | ^      | ipnet    | IP network

For config templates of Interface Classes or Connection Classes,
Relative prefix can be additionally used.

| Prefix   | Note
|:---------|:-------
| (none)   | Value of interface itself
| node_    | Node value of interface
| opp_     | Value of opposite interface
| oppnode_ | Node value of opposite interface

For example, {{ .opp_ipnet }} embeds IP network (such as "192.168.101.0/24")
of the opposite interface. 

The available numbers can be listed with "number" subcommand:

    dot2tinet number -c ./example/basic_bgp/bgp.yaml ./example/basic_bgp/bgp.dot


## Ordering

If the configuration have expected order coming from its dependency,
you can set priority values for config templates.
If priority value is larger, the config blocks will be on the head of merged configuration.
The default value of priority is 0,
which means you can also set negative values to place configs on the tail.


# TODO

- Add examples
- Add more supported NUMBERs
- l2vpn support (multi layered graph)
- source routing support (beacon labels)
- Config format check


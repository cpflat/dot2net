#!/usr/bin/env python
# -*- coding: utf-8 -*-

import sys

import click
import pygraphviz

DEFAULT_NODE_PREFIX = "node"
LABEL_KEY = "conf"

def generate_fabric(gname, rules):
    default_cnt = 0
    nodes = []
    for (n_nodes, prefix, labels) in rules:
        tier_nodes = []
        for i in range(n_nodes):
            if prefix is None:
                name = DEFAULT_NODE_PREFIX + str(default_cnt)
                default_cnt += 1
            else:
                name = prefix + str(i)
            tier_nodes.append((name, labels))
        nodes.append(tier_nodes)

    G = pygraphviz.AGraph(name=gname, directed=True)
    if len(nodes) == 0:
        return None

    for tier_nodes in nodes:
        for name, labels in tier_nodes:
            if len(labels) == 0:
                G.add_node(name)
            else:
                d = {LABEL_KEY: ";".join(labels)}
                G.add_nodes(name, **d)

    for i in range(len(nodes) - 1):
        tier_nodes = nodes[i]
        next_tier_nodes = nodes[i+1]
        for n1, _ in tier_nodes:
            for n2, _ in next_tier_nodes:
                G.add_edge(n1, n2, dir="none")
    
    return G


def parse_node_options(tierstr):
    tmp = tierstr.split(":")
    if len(tmp) == 1:
        return int(tmp[0]), None, []
    elif len(tmp) == 2:
        return int(tmp[0]), tmp[1], []
    else:
        return int(tmp[0]), tmp[1], tmp[2:]


@click.command()
@click.option("--name", default="", help="graph name")
@click.option(
    "--nodes", "-n", multiple=True, default=[],
    help=("Node tier definition. e.g., NUMBER:NAME:LABEL:LABEL:... "
          "NUMBER is the number of nodes. NAME is node name prefix. LABELs are annotated to the nodes.")
)
@click.option('--count', is_flag=True)
def main(name, nodes, count):
    rules = []
    for tierstr in nodes:
        rules.append(parse_node_options(tierstr))

    G = generate_fabric(name, rules)
    if count:
        print("{0} {1}".format(G.number_of_nodes(), G.number_of_edges()))
    else:
        print(G.string())


if __name__ == "__main__":
    main()    
    

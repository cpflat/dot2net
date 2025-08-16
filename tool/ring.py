#!/usr/bin/env python
# -*- coding: utf-8 -*-

import sys

import click
import pygraphviz

DEFAULT_NODE_PREFIX = "node"

def generate_ring(gname, n_nodes):
    if n_nodes <= 0:
        return None
    
    G = pygraphviz.AGraph(name=gname, directed=True)
    
    names = []
    for i in range(n_nodes):
        name = DEFAULT_NODE_PREFIX + str(i + 1)
        names.append(name)
        G.add_node(name)
    
    for i in range(n_nodes - 1):
        G.add_edge(names[i], names[i+1], dir="none")
    G.add_edge(names[-1], names[0], dir="none")
    
    return G


@click.command()
@click.option("--name", default="", help="graph name")
@click.option('--count', is_flag=True)
@click.argument("n_nodes", type=int)
def main(name, count, n_nodes):

    G = generate_ring(name, n_nodes)
    
    if count:
        print("{0} {1}".format(G.number_of_nodes(), G.number_of_edges()))
    else:
        print(G.string())


if __name__ == "__main__":
    main()    
    

#!/usr/bin/env python
# -*- coding: utf-8 -*-

import sys
import time
from collections import defaultdict
import subprocess
import numpy as np

if __name__ == "__main__":
    if len(sys.argv) == 1:
        sys.exit("usage: {0} NUMBER [ FILENAME ]".format(sys.argv[0]))
    elif len(sys.argv) == 2:
        source = sys.stdin
    else:
        source = open(sys.argv[2])
    num = int(sys.argv[1])

    commands = [line.rstrip("\n") for line in source]

    d = defaultdict(list)
    for cmd in commands:
        for i in range(num):
            start = time.time()
            subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, shell=True)
            end = time.time()
            d[cmd].append(end - start)
    
    for cmd in commands:
        values = np.array(d[cmd])
        print("# {0}".format(cmd))
        print("average: {0}".format(np.average(values)))
        print("std: {0}".format(np.std(values, ddof=1)))
        print("stderr: {0}".format(np.std(values, ddof=1) / np.sqrt(len(values))))
        print()

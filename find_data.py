#!/usr/bin/python3
# Using re to find data
import re

def open_file(filename):
    with open(filename) as file:
        lines = file.readlines()
    return lines

def compute_latency(lines):
    latency_line = {}
    line_count = 0
    for line in lines:
        if line.endswith('millseconds.\n'):
            latency_line[line_count] = line
            line_count += 1
    
    latency_total = 0
    for key in latency_line:
        latency_total += int(re.findall(r'\b\d+\b', latency_line[key])[-1])

    info_datas = re.findall(r'\b\d+\b', latency_line[0])
    print('K={} BatchSize={} Average Latencty={}'.format(info_datas[1], info_datas[2], latency_total/len(latency_line)))

if __name__ == '__main__':
    lines = open_file('log0.txt')
    compute_latency(lines)
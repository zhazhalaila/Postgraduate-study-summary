#!/usr/bin/python3
# Using re to find data
import re
import argparse

def open_file(filename):
    with open(filename) as file:
        lines = file.readlines()
    return lines

def compute_latency(lines, string):
    latency_line = {}
    line_count = 0
    for line in lines:
        if line.endswith(string):
            latency_line[line_count] = line
            line_count += 1
    
    latency_total = 0
    for key in latency_line:
        latency_total += int(re.findall(r'\b\d+\b', latency_line[key])[-1])

    info_datas = re.findall(r'\b\d+\b', latency_line[0])
    return info_datas, latency_total/len(latency_line)

if __name__ == '__main__':
    parser = argparse.ArgumentParser('start server')
    parser.add_argument('n', help='Total file to read.', type=int)
    parser.add_argument('p', help='Pattern to compute', type=int)
    args = parser.parse_args()
    pattern_map = {}
    pattern_map[1] = 'millseconds.\n'
    pattern_map[2] = 'bytes.\n'
    peer_latency = 0
    for i in range(args.n):
        lines = open_file('log'+str(i)+'.txt')
        info_datas, latency_average = compute_latency(lines, pattern_map[args.p])
        peer_latency += latency_average
        if args.p == 1:
            print('Peer={} K={} BatchSize={} Average Latency ={}millseconds'.format(i, info_datas[1], info_datas[2], latency_average))
        if args.p == 2:
            print('Peer={} K={} BatchSize={} Average Communication cost ={}bytes'.format(i, info_datas[1], info_datas[2], latency_average))
    print('Latency average = {}{}'.format(peer_latency/args.n, pattern_map[args.p]))
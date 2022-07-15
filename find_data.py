#!/usr/bin/python3
# Using re to find data
import re
import argparse

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
    return info_datas, latency_total/len(latency_line)

if __name__ == '__main__':
    parser = argparse.ArgumentParser('start server')
    parser.add_argument('n', help='Total file to read.', type=int)
    args = parser.parse_args()
    peer_latency = 0
    for i in range(args.n):
        lines = open_file('log'+str(i)+'.txt')
        info_datas, latency_average = compute_latency(lines)
        peer_latency += latency_average
        print('Peer={} K={} BatchSize={} Average Latencty={}'.format(i, info_datas[1], info_datas[2], latency_average))
    print('Latency average = {}millseconds'.format(peer_latency/args.n))
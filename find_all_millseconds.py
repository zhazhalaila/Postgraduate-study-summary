import re
import argparse

def open_file(filename):
    with open(filename) as file:
        lines = file.readlines()
    return lines

def read_millseconds(lines):
    for line in lines:
        if line.endswith("millseconds.\n"):
            print(line)

if __name__ == '__main__':
    parser = argparse.ArgumentParser('start server')
    parser.add_argument('n', help='Total file to read.', type=int)
    args = parser.parse_args()
    for i in range(args.n):
        lines = open_file('log'+str(i)+'.txt')
        read_millseconds(lines)
package readaddress

import (
	"bufio"
	"os"
)

func ReadAddress(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		n--
		if n < 0 {
			break
		}
		lines = append(lines, scanner.Text())
	}

	if scanner.Err() != nil {
		return nil, err
	}

	return lines, nil
}

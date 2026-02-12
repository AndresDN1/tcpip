package main

import (
	"bufio"
	"fmt"
	"strings"
)

func scanText(function bufio.SplitFunc) {
	r := strings.NewReader("This is\na string, a string in golang, the programming language, which will be scanned by the scanner")
	scanner := bufio.NewScanner(r)
	scanner.Split(function)
	_ = scanner.Scan()
	fmt.Println(scanner.Text())
	if scanner.Err() != nil {
		fmt.Println("Error scanning: ", scanner.Err().Error())
	}
}

func splitCommas(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := range data {
		if data[i] == ',' {
			return i + 1, data[:i+1], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data[:], nil
	}
	return 0, nil, nil
}

func main() {
	scanText(bufio.ScanLines)
	scanText(bufio.ScanWords)
	scanText(bufio.ScanRunes)
	scanText(bufio.ScanBytes)
	r := strings.NewReader("This is\na string, a string in golang, the programming language, which will be scanned by the scanner")
	scanner := bufio.NewScanner(r)
	scanner.Split(splitCommas)
	for scanner.Scan() {
		fmt.Println(string(scanner.Bytes()))
	}
	if scanner.Err() != nil {
		fmt.Println("Error scanning: ", scanner.Err().Error())
	}
}

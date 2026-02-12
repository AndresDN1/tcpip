package ip

import (
	"fmt"
)

type IPAddress [4]byte

func ParseIP(ipString string) (IPAddress, error) {
	var ip IPAddress

	octetIndex, currentOctet, octetValue := 0, 0, 0
	for i := range len(ipString) {
		char := ipString[i]

		if char == '.' {
			if currentOctet >= 3 {
				return ip, fmt.Errorf("invalid IP address: IPs must have 4 octets")
			}
			if octetIndex < 1 || octetIndex > 3 {
				return ip, fmt.Errorf("invalid IP address: octets must have between 1 and 3 digits")
			}
			ip[currentOctet] = byte(octetValue)
			currentOctet += 1
			octetValue, octetIndex = 0, 0
			continue
		}

		if char < '0' || char > '9' {
			return ip, fmt.Errorf("invalid IP address: invalid octet value")
		}

		if octetIndex == 1 && octetValue == 0 {
			return ip, fmt.Errorf("invalid IP address: leading zeros are not allowed")
		}

		octetValue *= 10
		octetValue += int(char - '0')
		octetIndex += 1

		if octetValue > 255 {
			return ip, fmt.Errorf("invalid IP address: invalid octet value")
		}
		if octetIndex > 3 {
			return ip, fmt.Errorf("invalid IP address: octets must have between 1 and 3 digits")
		}
	}

	if currentOctet != 3 {
		return ip, fmt.Errorf("invalid IP address: IP must have 4 octets")
	}
	if octetIndex < 1 {
		return ip, fmt.Errorf("invalid IP address: invalid octet length")
	}
	ip[currentOctet] = byte(octetValue)

	return ip, nil
}

/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package gpu

import (
	"regexp"
	"strconv"
)

type PCIAddress struct {
	Domain   int32
	Bus      int32
	Device   int32
	Function int32
}

var pcibusRegex = regexp.MustCompile(`(?i)([0-9a-f]{4,8})?:?([0-9a-f]{2,4}):([0-9a-f]{2})\.([0-9a-f])`)

func NewPCIAddressFromString(pciIdentifier string) PCIAddress {
	matches := pcibusRegex.FindAllStringSubmatch(pciIdentifier, -1)
	if len(matches) > 0 {
		domain, _ := strconv.ParseUint(matches[0][1], 16, 32)
		bus, _ := strconv.ParseUint(matches[0][2], 16, 32)
		device, _ := strconv.ParseUint(matches[0][3], 16, 32)
		function, _ := strconv.ParseUint(matches[0][4], 16, 32)

		return PCIAddress{
			Domain:   int32(domain),
			Bus:      int32(bus),
			Device:   int32(device),
			Function: int32(function),
		}
	}

	return PCIAddress{
		Domain:   -1,
		Bus:      -1,
		Device:   -1,
		Function: -1,
	}
}

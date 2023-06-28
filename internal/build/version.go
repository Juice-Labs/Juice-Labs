/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package build

import "fmt"

var (
	Major    = 0
	Minor    = 0
	Revision = 0

	Version = fmt.Sprintf("%d.%d.%d", Major, Minor, Revision)
)

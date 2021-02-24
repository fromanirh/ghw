//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package pci_test

import (
	"fmt"
	"testing"
)

// nolint: gocyclo
func TestPCICrosscheckSRIOV(t *testing.T) {
	info := pciTestSetup(t)

	tCases := []pciTestCase{
		{
			addr:    "0000:07:03.0",
			isSRIOV: false,
		},
		{
			addr:    "0000:05:11.0",
			isSRIOV: true,
		},
		{
			addr:    "0000:05:00.1",
			isSRIOV: true,
		},
	}
	for _, tCase := range tCases {
		t.Run(fmt.Sprintf("%s (sriov=%v)", tCase.addr, tCase.isSRIOV), func(t *testing.T) {
			dev := info.GetDevice(tCase.addr)
			if dev == nil {
				t.Fatalf("got nil device for address %q", tCase.addr)
			}
			sriovInfo := info.GetSRIOVInfo(tCase.addr)
			if tCase.isSRIOV && sriovInfo == nil {
				t.Fatalf("expected SRIOV info for device at address %q", tCase.addr)
			}
			if !tCase.isSRIOV && sriovInfo != nil {
				t.Fatalf("unexpected SRIOV info for device at address %q", tCase.addr)
			}
		})
	}

}

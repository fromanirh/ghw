//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package net

import (
	"fmt"

	"github.com/jaypipes/ghw/pkg/context"
	"github.com/jaypipes/ghw/pkg/marshal"
	"github.com/jaypipes/ghw/pkg/option"
)

type NICSRIOVInfo struct {
	PhysicalFunctionAddress  string   `json:"phys_fn_address,omitempty"`
	MaxVirtualFunctions      int      `json:"phys_max_vfs,omitempty"`
	VirtualFunctionAddresses []string `json:"virt_fn_addresses,omitempty"`
}

func (sriov *NICSRIOVInfo) IsPhysicalFunction() bool {
	return sriov != nil && (sriov.MaxVirtualFunctions > 0) && (sriov.PhysicalFunctionAddress == "")
}

func (sriov *NICSRIOVInfo) IsVirtualFunction() bool {
	return sriov != nil && (sriov.MaxVirtualFunctions == 0) && (sriov.PhysicalFunctionAddress != "")
}

type NICCapability struct {
	Name      string `json:"name"`
	IsEnabled bool   `json:"is_enabled"`
	CanEnable bool   `json:"can_enable"`
}

type NICAddress struct {
	PCI string `json:"pci"`
	// TODO(fromani): add other hw addresses (USB) when we support them
}

type NIC struct {
	Name         string           `json:"name"`
	MacAddress   string           `json:"mac_address"`
	IsVirtual    bool             `json:"is_virtual"`
	Capabilities []*NICCapability `json:"capabilities"`
	HWAddress    NICAddress       `json:"hw_address"`
	SRIOVInfo    *NICSRIOVInfo    `json:"sriov,omitempty"`
}

func (n *NIC) String() string {
	isVirtualStr := ""
	if n.IsVirtual {
		isVirtualStr = " (virtual)"
	}
	return fmt.Sprintf(
		"%s%s",
		n.Name,
		isVirtualStr,
	)
}

type Info struct {
	ctx  *context.Context
	NICs []*NIC `json:"nics"`
}

// New returns a pointer to an Info struct that contains information about the
// network interface controllers (NICs) on the host system
func New(opts ...*option.Option) (*Info, error) {
	ctx := context.New(opts...)
	info := &Info{ctx: ctx}
	if err := ctx.Do(info.load); err != nil {
		return nil, err
	}
	return info, nil
}

func (i *Info) String() string {
	return fmt.Sprintf(
		"net (%d NICs)",
		len(i.NICs),
	)
}

// simple private struct used to encapsulate net information in a
// top-level "net" YAML/JSON map/object key
type netPrinter struct {
	Info *Info `json:"network"`
}

// YAMLString returns a string with the net information formatted as YAML
// under a top-level "net:" key
func (i *Info) YAMLString() string {
	return marshal.SafeYAML(i.ctx, netPrinter{i})
}

// JSONString returns a string with the net information formatted as JSON
// under a top-level "net:" key
func (i *Info) JSONString(indent bool) string {
	return marshal.SafeJSON(i.ctx, netPrinter{i}, indent)
}

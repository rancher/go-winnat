package winnat

import (
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"

	"github.com/orangedeng/go-winnat/drivers"
)

type NatDriver interface {
	Init(map[string]interface{}) error
	CreatePortMapping(externalIP net.IP,
		externalPort uint32,
		internalIP net.IP,
		internalPort uint32,
		Protocol string) (drivers.PortMapping, error)
	ListPortMapping() ([]drivers.PortMapping, error)
	DeletePortMapping(drivers.PortMapping) error
	Destory() error
}

func NewNatDriver(driverName string, config map[string]interface{}) (NatDriver, error) {
	logrus.Info("get in")
	var rtn NatDriver
	switch driverName {
	// case drivers.WinNatDriverName:
	// 	rtn = &drivers.WinNat{}
	case drivers.NetshDriverName:
		rtn = &drivers.Netsh{}
	default:
		return nil, fmt.Errorf("driver name %s is not supported", driverName)
	}
	err := rtn.Init(config)
	return rtn, err
}

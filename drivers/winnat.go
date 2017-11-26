package drivers

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"reflect"

	ps "github.com/gorillalabs/go-powershell"
	psbe "github.com/gorillalabs/go-powershell/backend"
)

const (
	WinNatDriverName      = "WinNAT"
	defaultNatName        = "rancher"
	listPortMappingByName = "Get-NetNatStaticMapping -NatName %s  |Sort-Object -Property StaticMappingID |format-list -Property *"
	addPortMapping        = "New-NetNatStaticMapping %s | format-list -Property *"
	deletePortMappingByID = "Get-NetNatStaticMapping -StaticMappingID %d | Remove-NetNatStaticMapping -Confirm false"
	deleteAllPortMapping  = "Get-NetNatStaticMapping | Remove-NetNatStaticMapping -Confirm false"
)

type WinNatPortMapping struct {
	Protocol                      string     `mapstructure:"Procotol" powershell:",get;set;"`
	Active                        string     `mapstructure:"Active" powershell:",get;set;"`
	Caption                       string     `mapstructure:"Caption" powershell:",get;set;"`
	Description                   string     `mapstructure:"Description" powershell:",get;set;"`
	ElementName                   string     `mapstructure:"ElementName" powershell:",get;set;"`
	InstanceID                    string     `mapstructure:"InstanceID" powershell:",get;set;"`
	ExternalIPAddress             net.IP     `mapstructure:"ExternalIPAddress" powershell:",get;set;"`
	ExternalPort                  uint       `mapstructure:"ExternalPort" powershell:",get;set;"`
	InternalIPAddress             net.IP     `mapstructure:"InternalIPAddress" powershell:",get;set;"`
	InternalPort                  uint       `mapstructure:"InternalPort" powershell:",get;set;"`
	InternalRoutingDomainID       string     `mapstructure:"InternalRoutingDomainId" powershell:",get;set;"`
	NatName                       string     `mapstructure:"NatName" powershell:",get;"`
	RemoteExternalIPAddressPrefix *net.IPNet `mapstructure:"RemoteExternalIPAddressPrefix" powershell:",get;set;"`
	StaticMappingID               uint64     `mapstructure:"StaticMappingID" powershell:",get;"`
}

type WinNat struct {
	shell ps.Shell
}

func (driver *WinNat) Init(config map[string]interface{}) error {
	var err error
	driver.shell, err = ps.New(&psbe.Local{})
	if err != nil {
		return err
	}
	return nil
}
func (driver *WinNat) CreatePortMapping(externalIP net.IP,
	externalPort uint32,
	internalIP net.IP,
	internalPort uint32,
	Protocol string) (PortMapping, error) {
	return PortMapping{}, nil
}
func (driver *WinNat) ListPortMapping() ([]PortMapping, error) {
	return nil, nil
}
func (driver *WinNat) DeletePortMapping(PortMapping) error {
	return nil
}
func (driver *WinNat) Destory() error {
	return nil
}

func (rule *WinNatPortMapping) toNewString() (string, error) {
	if rule.NatName == "" {
		return "", errors.New("NatName is required for adding rule")
	}
	if rule.RemoteExternalIPAddressPrefix == nil {
		return "", errors.New("RemoteExternalIPAddressPrefix is required for adding rule")
	}
	if rule.InternalPort == 0 || rule.InternalPort > 65535 {
		return "", errors.New("InternalPort is not valid")
	}
	if rule.ExternalPort == 0 || rule.ExternalPort > 65535 {
		return "", errors.New("ExternalPort is not valid")
	}
	if rule.InternalIPAddress.Equal(net.ParseIP("0.0.0.0")) || rule.InternalIPAddress.Equal(net.ParseIP("::")) {
		return "", errors.New("InternalIPAddress is required for adding rule")
	}
	rtn := bytes.NewBuffer([]byte{})
	rtn.WriteString(" -NatName " + rule.NatName)
	rtn.WriteString(ToPowershellString(rule))
	return rtn.String(), nil
}

func RemoveLocalPortMapping(shell ps.Shell) error {
	_, _, err := shell.Execute(deleteAllPortMapping)
	if err != nil {
		return err
	}
	return nil
}

func ListLocalPortMapping(shell ps.Shell) ([]*WinNatPortMapping, error) {
	cmd := fmt.Sprintf(listPortMappingByName, defaultNatName)
	output, _, err := shell.Execute(cmd)
	if err != nil {
		return nil, err
	}
	is, err := parseRows(output, reflect.TypeOf(WinNatPortMapping{}))
	if err != nil {
		return nil, err
	}
	return parseArray(is)
}

func (rule *WinNatPortMapping) Add(shell ps.Shell) (*WinNatPortMapping, error) {
	cmd, err := rule.GetAddCommand()
	if err != nil {
		return nil, err
	}
	output, _, err := shell.Execute(cmd)
	if err != nil {
		return nil, err
	}
	rtn, err := parseRow([]byte(output), reflect.TypeOf(rule))
	if err != nil {
		return nil, err
	}
	return rtn.(*WinNatPortMapping), nil
}

func (rule *WinNatPortMapping) GetAddCommand() (string, error) {
	newString, err := rule.toNewString()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(addPortMapping, newString), nil
}

func (rule *WinNatPortMapping) Delete(shell ps.Shell) error {
	if rule.StaticMappingID == 0 {
		return errors.New("StaticMappingID is not valid")
	}
	if _, _, err := shell.Execute(fmt.Sprintf(deletePortMappingByID, rule.StaticMappingID)); err != nil {
		return err
	}
	return nil
}

func (rule *WinNatPortMapping) Equal(target *WinNatPortMapping) bool {
	if target == nil {
		return false
	}
	return rule.ExternalPort == target.ExternalPort &&
		rule.ExternalIPAddress.Equal(target.ExternalIPAddress) &&
		rule.InternalPort == target.InternalPort &&
		rule.ExternalIPAddress.Equal(target.ExternalIPAddress) &&
		rule.Protocol == target.Protocol
}

func parseArray(input []interface{}) ([]*WinNatPortMapping, error) {
	rtns := make([]*WinNatPortMapping, len(input))
	for i := 0; i < len(input); i++ {
		if _, ok := input[i].(*WinNatPortMapping); !ok {
			return nil, errors.New("Type is mismatch")
		}
		rtns[i] = input[i].(*WinNatPortMapping)
	}
	return rtns, nil
}

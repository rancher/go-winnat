package drivers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
)

const (
	NetshDriverName = "Netsh"
	NatAdapterName  = "NatAdapter"
	seperator       = `--+\n`
)

type Netsh struct {
	adapterName string
}

func (driver *Netsh) Init(config map[string]interface{}) error {
	v, ok := config[NatAdapterName]
	if !ok {
		return fmt.Errorf("configuration missing %s", NatAdapterName)
	}
	if _, ok = v.(string); ok {
		driver.adapterName = v.(string)
	} else if _, ok = v.(net.Interface); ok {
		driver.adapterName = (v.(net.Interface)).Name
	} else {
		return fmt.Errorf("configuration %s value is not valid", NatAdapterName)
	}
	driver.adapterName = `"` + driver.adapterName + `"`
	_, err := driver.ListPortMapping()
	if err != nil {
		return errors.New("NAT is not setuped")
	}
	return nil
}

func (driver *Netsh) CreatePortMapping(input PortMapping) (PortMapping, error) {
	errbuff := bytes.NewBuffer([]byte{})
	outbuff := bytes.NewBuffer([]byte{})
	cmd := exec.Command(
		"netsh",
		"routing",
		"ip",
		"nat",
		"add",
		"portmapping",
		driver.adapterName,
		input.Protocol,
		input.ExternalIP.String(),
		strconv.FormatUint(uint64(input.ExternalPort), 10),
		input.InternalIP.String(),
		strconv.FormatUint(uint64(input.InternalPort), 10),
	)
	cmd.Stderr = errbuff
	cmd.Stdout = outbuff
	err := cmd.Run()
	if err != nil {
		logrus.Error(outbuff.String())
		logrus.Error(errbuff.String())
	}
	return input, err
}

func (driver *Netsh) CreatePortMappings(inputs []PortMapping) error {
	cmds := driver.parseAddCmd(inputs[0])
	for i := 1; i < len(inputs); i++ {
		//windows command seperator
		cmds = append(cmds, "&&")
		cmds = append(cmds, driver.parseAddCmd(inputs[i])...)
	}
	return exec.Command(cmds[0], cmds[1:]...).Run()
}

func (driver *Netsh) parseAddCmd(input PortMapping) []string {
	return []string{
		"netsh",
		"routing",
		"ip",
		"nat",
		"add",
		"portmapping",
		driver.adapterName,
		input.Protocol,
		input.ExternalIP.String(),
		strconv.FormatUint(uint64(input.ExternalPort), 10),
		input.InternalIP.String(),
		strconv.FormatUint(uint64(input.InternalPort), 10),
	}
}

func (driver *Netsh) ListPortMapping() ([]PortMapping, error) {
	outputBuffer := bytes.NewBuffer([]byte{})
	cmd := exec.Command("netsh", "routing", "ip", "nat", "show", "interface", driver.adapterName)
	cmd.Stdout = outputBuffer
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	output := strings.Replace(outputBuffer.String(), "\r", "", -1)
	rexp := regexp.MustCompile(seperator)
	blocks := rexp.Split(output, -1)
	switch len(blocks) {
	case 1: //when only one element in the array, it doesn't have any nat interface in RRAS service
		return nil, fmt.Errorf("%s is not a nat interface", driver.adapterName)
	case 2: //when there are two element in the array, there is nothing in the PortMapping list
		return []PortMapping{}, nil
	case 3: //there are some port mapping rules in this interface
	default:
		logrus.Debug(output)
		return nil, errors.New("driver Netsh error, ListPortMapping get unexpected output")
	}

	//one portmapping object will be like following
	/*
		protocol    : TCP
		publicip    : 0.0.0.0
		publicport  : 80
		privateip   : 192.169.1.100
		privateport : 80
	*/
	scanner := bufio.NewScanner(strings.NewReader(blocks[2]))
	current := &PortMapping{}
	currentLineCount := 0
	rtn := []PortMapping{}
	//scanning portmapping objects to struct
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			rtn = append(rtn, *current)
			current = &PortMapping{}
			currentLineCount = 0
			continue
		}
		value := strings.TrimSpace(strings.Split(line, ":")[1])
		switch currentLineCount {
		case 0:
			current.Protocol = value
		case 1:
			current.ExternalIP = net.ParseIP(value)
		case 2:
			port, _ := strconv.Atoi(value)
			current.ExternalPort = uint32(port)
		case 3:
			current.InternalIP = net.ParseIP(value)
		case 4:
			port, _ := strconv.Atoi(value)
			current.InternalPort = uint32(port)
		}
		currentLineCount++
	}
	return rtn, nil
}

func (driver *Netsh) DeletePortMapping(tar PortMapping) error {
	return exec.Command(
		"netsh",
		"routing",
		"ip",
		"nat",
		"delete",
		"portmapping",
		driver.adapterName,
		tar.Protocol,
		tar.ExternalIP.String(),
		strconv.FormatUint(uint64(tar.ExternalPort), 10),
	).Run()
}

func (driver *Netsh) DeletePortMappings(inputs []PortMapping) error {
	cmds := driver.parseDeleteCmd(inputs[0])
	for i := 1; i < len(inputs); i++ {
		//windows command seperator
		cmds = append(cmds, "&&")
		cmds = append(cmds, driver.parseDeleteCmd(inputs[i])...)
	}
	return exec.Command(cmds[0], cmds[1:]...).Run()
}

func (driver *Netsh) parseDeleteCmd(input PortMapping) []string {
	return []string{
		"netsh",
		"routing",
		"ip",
		"nat",
		"delete",
		"portmapping",
		driver.adapterName,
		input.Protocol,
		input.ExternalIP.String(),
		strconv.FormatUint(uint64(input.ExternalPort), 10),
	}
}

func (driver *Netsh) Destory() error {
	return nil
}

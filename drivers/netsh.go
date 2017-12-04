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
	adapterNames []string
}

func (driver *Netsh) Init(config map[string]interface{}) error {
	v, ok := config[NatAdapterName]
	if !ok {
		return fmt.Errorf("configuration missing %s", NatAdapterName)
	}
	if _, ok = v.(string); ok {
		driver.adapterNames = strings.Split(v.(string), ",")
	} else {
		return fmt.Errorf("configuration %s value is not valid", NatAdapterName)
	}
	_, err := driver.ListPortMapping()
	if err != nil {
		return errors.New("NAT is not setuped")
	}
	return nil
}

func (driver *Netsh) CreatePortMapping(input PortMapping) (PortMapping, error) {
	return input, driver.CreatePortMappings([]PortMapping{input})
}

func (driver *Netsh) CreatePortMappings(inputs []PortMapping) (err error) {
	if len(inputs) == 0 {
		return nil
	}
	for _, adapterName := range driver.adapterNames {
		for i := 0; i < len(inputs); i++ {
			errbuff := bytes.NewBuffer([]byte{})
			outbuff := bytes.NewBuffer([]byte{})
			//windows command seperator
			cmds := parseAddCmd(adapterName, inputs[i])
			cmd := exec.Command(cmds[0], cmds[1:]...)
			cmd.Stderr = errbuff
			cmd.Stdout = outbuff
			if err = cmd.Run(); err != nil {
				logrus.Error(err)
			}
			logrus.Infof("run adding port mapping %s %d %s output is %s",
				inputs[i].ExternalIP.String(),
				inputs[i].ExternalPort,
				adapterName,
				outbuff.String(),
			)
			if outbuff.Len() > 2 {
				err = errors.New(outbuff.String())
			}
		}
	}
	return err
}

func parseAddCmd(adapterName string, input PortMapping) (rtn []string) {
	defer func() {
		logrus.Infof("%v", rtn)
	}()
	return []string{
		"netsh",
		"routing",
		"ip",
		"nat",
		"add",
		"portmapping",
		adapterName,
		input.Protocol,
		input.ExternalIP.String(),
		strconv.FormatUint(uint64(input.ExternalPort), 10),
		input.InternalIP.String(),
		strconv.FormatUint(uint64(input.InternalPort), 10),
	}
}

func (driver *Netsh) ListPortMapping() ([]PortMapping, error) {
	outputBuffer := bytes.NewBuffer([]byte{})
	cmd := exec.Command("netsh", "routing", "ip", "nat", "show", "interface", driver.adapterNames[0])
	cmd.Stdout = outputBuffer
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	output := strings.Replace(outputBuffer.String(), "\r", "", -1)
	rexp := regexp.MustCompile(seperator)
	blocks := rexp.Split(output, -1)
	switch len(blocks) {
	case 1: //when only one element in the array, it doesn't have any nat interface in RRAS service
		return nil, fmt.Errorf("%s is not a nat interface", driver.adapterNames[0])
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
	return driver.DeletePortMappings([]PortMapping{tar})
}

func (driver *Netsh) DeletePortMappings(inputs []PortMapping) (err error) {
	if len(inputs) == 0 {
		return nil
	}
	for _, adapterName := range driver.adapterNames {
		for i := 0; i < len(inputs); i++ {
			errbuff := bytes.NewBuffer([]byte{})
			outbuff := bytes.NewBuffer([]byte{})
			//windows command seperator
			cmds := parseDeleteCmd(adapterName, inputs[i])
			cmd := exec.Command(cmds[0], cmds[1:]...)
			cmd.Stderr = errbuff
			cmd.Stdout = outbuff
			if err = cmd.Run(); err != nil {
				logrus.Error(err)
			}
			logrus.Infof("run deleting port mapping %s %d %s output is %s",
				inputs[i].ExternalIP.String(),
				inputs[i].ExternalPort,
				adapterName,
				outbuff.String(),
			)
			//outbuff return \r\n when success
			if outbuff.Len() > 2 {
				err = errors.New(outbuff.String())
			}
		}
	}
	return err
}

func parseDeleteCmd(adapterName string, input PortMapping) (rtn []string) {
	defer func() {
		logrus.Debug("%v", rtn)
	}()
	return []string{
		"netsh",
		"routing",
		"ip",
		"nat",
		"delete",
		"portmapping",
		adapterName,
		input.Protocol,
		input.ExternalIP.String(),
		strconv.FormatUint(uint64(input.ExternalPort), 10),
	}
}

func (driver *Netsh) Destory() error {
	return nil
}

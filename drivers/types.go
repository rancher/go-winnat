package drivers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	ps "github.com/gorillalabs/go-powershell"
	"github.com/mitchellh/mapstructure"
)

type PortMapping struct {
	ExternalID   string
	ExternalIP   net.IP
	ExternalPort uint32
	InternalIP   net.IP
	InternalPort uint32
	Protocol     string
}

func (p *PortMapping) Equal(tp *PortMapping) bool {
	return tp != nil &&
		strings.ToLower(p.Protocol) == strings.ToLower(tp.Protocol) &&
		p.ExternalIP.Equal(tp.ExternalIP) &&
		p.ExternalPort == tp.ExternalPort &&
		p.InternalIP.Equal(tp.InternalIP) &&
		p.InternalPort == tp.InternalPort
}

const (
	defaultPowershellSeparator = ";"
)

type PowershellBatch struct {
	shell        ps.Shell
	batchContent []string
	executed     bool
}

func NewPowershellBatch(shell ps.Shell) *PowershellBatch {
	return &PowershellBatch{
		shell:        shell,
		batchContent: []string{},
		executed:     false,
	}
}

func (p *PowershellBatch) Execute() (int, error) {
	defer func() { p.executed = true }()
	for count := 0; count < len(p.batchContent); count++ {
		if _, _, err := p.shell.Execute(p.batchContent[count]); err != nil {
			return count, err
		}
	}
	return len(p.batchContent), nil
}

func (p *PowershellBatch) ExecuteFast() error {
	defer func() { p.executed = true }()
	cmd := strings.Join(p.batchContent, defaultPowershellSeparator)
	_, _, err := p.shell.Execute(cmd)
	return err
}

func (p *PowershellBatch) Append(content string) {
	p.batchContent = append(p.batchContent, content)
}

func (p *PowershellBatch) IsExecuted() bool {
	return p.executed
}

func (p *PowershellBatch) Reset() {
	p.batchContent = []string{}
	p.executed = false
}

func ToPowershellString(target interface{}) string {
	buff := bytes.NewBuffer([]byte{})
	value := reflect.ValueOf(target)
	value = value.Elem()
	vt := value.Type()
	for i := 0; i < vt.NumField(); i++ {
		ft := vt.Field(i)
		tname, _, set := powershellTagParse(ft.Tag.Get("powershell"))
		field := value.Field(i)
		if tname == "" {
			tname = ft.Name
		}
		if !set {
			continue
		}
		switch field.Type() {
		case reflect.TypeOf(&net.IPNet{}):
			if field.IsNil() {
				continue
			}
			tar := field.Interface().(*net.IPNet)
			buff.WriteString(fmt.Sprintf(" -%s %s", tname, tar.String()))
		case reflect.TypeOf(net.ParseIP("0.0.0.0")):
			tar := field.Interface().(net.IP)
			buff.WriteString(fmt.Sprintf(" -%s %s", tname, tar.String()))
		}
		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				continue
			}
			buff.WriteString(fmt.Sprintf(" -%s %s", tname, field.String()))
		case reflect.Uint64:
			buff.WriteString(fmt.Sprintf(" -%s %d", tname, field.Uint()))
		}
	}
	return buff.String()
}

func parseRows(input string, objType reflect.Type) ([]interface{}, error) {
	if objType.Kind() == reflect.Ptr {
		objType = objType.Elem()
	}
	_input := strings.Replace(input, "\r", "", -1)
	ss := strings.Split(_input, "\n")
	var rows []interface{}
	buff := bytes.NewBufferString("")
	for _, s := range ss {
		if len(s) == 0 && buff.Len() != 0 {
			row, err := parseRow(buff.Bytes(), objType)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
			buff.Reset()
		} else if len(s) != 0 {
			buff.WriteString(s + "\n")
		}
	}
	if buff.Len() != 0 {
		row, err := parseRow(buff.Bytes(), objType)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		buff.Reset()
	}
	return rows, nil
}

func map2Row(targetMap map[string]interface{}, obj interface{}) error {
	config := &mapstructure.DecoderConfig{
		DecodeHook: stringToIPDecodeHookFunc,
		Metadata:   nil,
		Result:     obj,
		ZeroFields: false,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}
	return decoder.Decode(targetMap)
}

func parseRow(input []byte, objType reflect.Type) (interface{}, error) {
	if objType.Kind() == reflect.Ptr {
		objType = objType.Elem()
	}
	tmpMap := map[string]interface{}{}
	scanner := bufio.NewScanner(bytes.NewReader(input))
	lastKey := ""
	seperatorIndex := bytes.IndexRune(input, ':')
	for scanner.Scan() {
		line := scanner.Text()
		kvpair := strings.SplitN(line, ":", 2)
		switch len(kvpair) {
		case 1:
			if lastKey == "" {
				return nil, errors.New(line + " is not a valid output")
			}
			tmpMap[lastKey] = tmpMap[lastKey].(string) + line[seperatorIndex+2:len(line)]
		case 2:
			tmpMap[strings.TrimSpace(kvpair[0])] = strings.TrimSpace(kvpair[1])
			lastKey = strings.TrimSpace(kvpair[0])
		default:
			return nil, errors.New(line + " is not a valid output")
		}
	}
	rule := reflect.New(objType).Interface()
	err := map2Row(tmpMap, rule)
	return rule, err
}

//stringToIPDecodeHookFunc t1 is string, t2 is ip/ipnet
func stringToIPDecodeHookFunc(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() != reflect.String {
		return data, nil
	}
	switch t.Kind() {
	case reflect.Uint64:
		return strconv.ParseUint(data.(string), 10, 0)
	}
	switch t {
	case reflect.TypeOf(net.ParseIP("0.0.0.0")):
		rtn := net.ParseIP(data.(string))
		if rtn.String() != data.(string) {
			return data, errors.New("ip address " + data.(string) + " is not valid")
		}
		return rtn, nil
	case reflect.TypeOf(&net.IPNet{}):
		_, rtn, err := net.ParseCIDR(data.(string))
		return rtn, err
	default:
		return data, nil
	}
}

func powershellTagParse(tarvalue string) (Name string, get, set bool) {
	tmp := strings.Split(tarvalue, ",")
	Name = tmp[0]
	if len(tmp) == 2 {
		if strings.Index(tmp[1], "get;") != -1 {
			get = true
		}
		if strings.Index(tmp[1], "set;") != -1 {
			set = true
		}
		return
	}
	return
}

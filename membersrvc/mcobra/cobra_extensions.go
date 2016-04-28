package mcobra

import (
 "github.com/spf13/pflag"
 pb "github.com/hyperledger/fabric/membersrvc/protos"

 "strings"
)

type systemRoleValue int32

func (t *systemRoleValue) String() string {
	return pb.Role_name[int32(*t)]
}

func (t *systemRoleValue) Set(val string) error {
	*t = (systemRoleValue)(getRoleValue(val))
	return nil
}

func (t *systemRoleValue) Type() string {
	return "SystemRole"
}

func getRoleValue(val string) int32 {
	valStr := strings.ToUpper(val)
	return pb.Role_value[valStr]
}

func newSystemRoleValue(val string, p *int32) (*systemRoleValue) {
	*p = getRoleValue(val)
	return (*systemRoleValue)(p)
}



func AddSystemRoleP(f *pflag.FlagSet, p *int32, name, shorthand string, value, usage string) {
	flag := f.VarPF(newSystemRoleValue(value, p), name, shorthand, usage)
	flag.NoOptDefVal = value
}

package common

import "strconv"

// RoleType
type RoleType uint32

const (
	RoleNil             RoleType = 0x001
	RoleDefault                  = 0x002
	RoleBucket                   = 0x004
	RoleBackupMiner              = 0x008
	RoleMiner                    = 0x010
	RoleInnerMiner               = 0x020
	RoleBackupValidator          = 0x040
	RoleValidator                = 0x080
	RoleBackupBroadcast          = 0x100
	RoleBroadcast                = 0x200
	RoleAll                      = 0xFFFF
)

func (rt RoleType) String() string {
	switch rt {
	case RoleNil:
		return "nil"
	case RoleDefault:
		return "default"
	case RoleBucket:
		return "bucket"
	case RoleBackupMiner:
		return "backup miner"
	case RoleMiner:
		return "miner"
	case RoleInnerMiner:
		return "inner miner"
	case RoleBackupValidator:
		return "backup validator"
	case RoleValidator:
		return "validator"
	case RoleBackupBroadcast:
		return "backup broadcast"
	case RoleBroadcast:
		return "broadcast"
	default:
		return strconv.Itoa(int(rt))
	}
}

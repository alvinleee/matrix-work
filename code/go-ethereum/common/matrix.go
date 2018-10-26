package common

//RoleType
//type RoleType uint32

/*
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
)*/

type ElectRoleType uint8

const (
	ElectRoleMiner           ElectRoleType = 0x00
	ElectRoleMinerBackUp     ElectRoleType = 0x01
	ElectRoleValidator       ElectRoleType = 0x02
	ElectRoleValidatorBackUp ElectRoleType = 0x03
)

func (ert ElectRoleType) Transfer2CommonRole() RoleType {
	switch ert {
	case ElectRoleMiner:
		return RoleMiner
	case ElectRoleMinerBackUp:
		return RoleBackupMiner
	case ElectRoleValidator:
		return RoleValidator
	case ElectRoleValidatorBackUp:
		return RoleBackupValidator
	}
	return RoleNil
}

func GetRoleTypeFromPosition(position uint16) RoleType {
	return ElectRoleType(position >> 12).Transfer2CommonRole()
}

func GeneratePosition(index uint16, electRole ElectRoleType) uint16 {
	return uint16(electRole)<<12 + index
}

type Position struct {
	init int
	now  int
}

const (
	MasterValidatorNum = 11
	BackupValidatorNum = 3
)

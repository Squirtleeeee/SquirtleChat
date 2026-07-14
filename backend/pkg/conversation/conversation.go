package conversation

import (
	"fmt"
	"strconv"
)

func DirectID(uid1, uid2 int64) string {
	if uid1 > uid2 {
		uid1, uid2 = uid2, uid1
	}
	return fmt.Sprintf("d_%d_%d", uid1, uid2)
}

func GroupID(groupID int64) string {
	return "g_" + strconv.FormatInt(groupID, 10)
}

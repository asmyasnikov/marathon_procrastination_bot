package env

import (
	"os"
	"strconv"
)

const (
	YDB_CONNECTION_STRING = "YDB_CONNECTION_STRING"
	TELEGRAM_TOKEN        = "TELEGRAM_TOKEN"
	MAGIC_NUMBER          = "MAGIC_NUMBER"
	FREEZE_HOURS          = "FREEZE_HOURS"

	magicNumber = 347863284
	freezeHours = 15
)

func Magic() int {
	if v, has := os.LookupEnv(MAGIC_NUMBER); !has {
		return magicNumber
	} else if vv, err := strconv.Atoi(v); err != nil {
		return magicNumber
	} else {
		return vv
	}
}

func FreezeHours() int {
	if v, has := os.LookupEnv(FREEZE_HOURS); !has {
		return freezeHours
	} else if vv, err := strconv.Atoi(v); err != nil {
		return freezeHours
	} else {
		return vv
	}
}

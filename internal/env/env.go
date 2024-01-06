package env

import (
	"os"
	"strconv"
)

const (
	YDB_CONNECTION_STRING = "YDB_CONNECTION_STRING"
	TELEGRAM_TOKEN        = "TELEGRAM_TOKEN"
	MAGIC_NUMBER          = 347863284
)

func Magic() int {
	if v, has := os.LookupEnv("MAGIC_NUMBER"); !has {
		return MAGIC_NUMBER
	} else if vv, err := strconv.Atoi(v); err != nil {
		return MAGIC_NUMBER
	} else {
		return vv
	}
}

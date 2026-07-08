package data

import (
	"fmt"
	"os"
)

func WriteToText(file os.File, data string) bool {
	if _, err := file.WriteString(data); err != nil {
		fmt.Print(fmt.Errorf("%s", err))
		return false
	}

	return true
}